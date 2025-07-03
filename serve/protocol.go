package serve

import (
	"archive/tar"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tsukinoko-kun/pogo/db"
	"github.com/tsukinoko-kun/pogo/protos"
	"github.com/tsukinoko-kun/pogo/repos"
	"github.com/tsukinoko-kun/pogo/serve/serveerrors"
	"github.com/tsukinoko-kun/pogo/signedhttp"
	"github.com/tsukinoko-kun/pogo/utils"

	"google.golang.org/protobuf/proto"
)

func (a *App) handleRpc(w http.ResponseWriter, httpReq *http.Request) {
	defer httpReq.Body.Close()

	r, err := signedhttp.NewRequest(httpReq)
	if err != nil {
		if errors.Is(err, signedhttp.ErrMissingSignature) ||
			errors.Is(err, signedhttp.ErrInvalidSignature) ||
			errors.Is(err, signedhttp.ErrSignatureVerificationFailed) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Close()

	funcName := r.PathValue("func")
	switch funcName {
	case "push":
		a.handlePush(w, r)
	case "check_files_exists":
		a.handleCheckFilesExists(w, r)
	case "new_change":
		a.handleNewChange(w, r)
	case "checkout":
		a.handleCheckout(w, r)
	case "set_bookmark":
		a.handleSetBookmark(w, r)
	case "log":
		a.handleLog(w, r)
	case "conflicts":
		a.handleConflicts(w, r)
	case "find_change":
		a.handleFindChange(w, r)
	case "describe":
		a.handleDescribe(w, r)
	case "list_bookmarks":
		a.handleListBookmarks(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (a *App) openRepo(idStr string) (repos.Repo, error) {
	if id, err := strconv.ParseInt(idStr, 10, 32); err == nil {
		return repos.Open(int32(id)), nil
	}
	// try by name as fallback
	return repos.OpenByName(idStr)
}

var repoNamingRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

func (a *App) handleInit(w http.ResponseWriter, httpReq *http.Request) {
	defer httpReq.Body.Close()

	r, err := signedhttp.NewRequest(httpReq)
	if err != nil {
		if errors.Is(err, signedhttp.ErrMissingSignature) ||
			errors.Is(err, signedhttp.ErrInvalidSignature) ||
			errors.Is(err, signedhttp.ErrSignatureVerificationFailed) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Close()

	req := new(protos.InitRequest)
	err = protos.Unmarshal(r.Body(), req)
	if err != nil {
		http.Error(w, "unmarshal init request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if !repoNamingRegex.MatchString(req.Name) {
		http.Error(w, "invalid repo name '"+req.Name+"': must start with a letter and only contain letters, numbers, dashes, and underscores", http.StatusBadRequest)
		return
	}

	tx, err := db.Q.Begin(r.Context())
	if err != nil {
		http.Error(w, "begin transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Close()

	repo, err := repos.Create(tx, req.Name)
	if err != nil {
		http.Error(w, "create repository db: "+err.Error(), http.StatusInternalServerError)
		return
	}

	changeName, err := tx.GenerateChangeName(r.Context(), repo.ID())
	if err != nil {
		http.Error(w, "generate change name: "+err.Error(), http.StatusInternalServerError)
		return
	}

	changeId, err := tx.CreateChange(
		r.Context(),
		repo.ID(),
		changeName,
		utils.Ptr("init"),
		r.Username(),
		r.MachineID(),
		0,
	)
	if err != nil {
		http.Error(w, "create 'init' change: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, bookmark := range req.Bookmarks {
		if err = tx.SetBookmark(r.Context(), repo.ID(), bookmark, changeId); err != nil {
			http.Error(
				w,
				fmt.Sprintf("set bookmark '%s' to change %d: %s", bookmark, changeId, err.Error()),
				http.StatusInternalServerError,
			)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	initResp := &protos.InitResponse{
		RepoID: int32(repo),
	}
	_ = protos.MarshalWrite(initResp, w)
}

func (a *App) handlePush(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tx, err := db.Q.Begin(r.Context())
	if err != nil {
		http.Error(w, "begin transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Close()

	changeName := r.Header.Get("X-Change-Name")

	changeId, err := tx.FindChange(r.Context(), repo.ID(), changeName)
	if err != nil {
		http.Error(w, "find change '"+changeName+"': "+err.Error(), http.StatusInternalServerError)
		return
	}

	if hasChild, err := tx.HasChangeChild(r.Context(), utils.Ptr(changeId)); err != nil {
		http.Error(w, "check if change has child: "+err.Error(), http.StatusInternalServerError)
		return
	} else if hasChild {
		http.Error(w, serveerrors.ErrPushToChangeWithChild.Error(), http.StatusBadRequest)
		return
	}

	if owner, err := tx.GetChangeOwner(r.Context(), changeId); err != nil {
		http.Error(w, "get change owner: "+err.Error(), http.StatusInternalServerError)
		return
	} else if owner.Author != r.Username() || owner.Device != r.MachineID() {
		http.Error(w, serveerrors.ErrPushToChangeNotOwned.Error(), http.StatusBadRequest)
		return
	}

	if err = tx.ClearChange(r.Context(), changeId); err != nil {
		http.Error(w, "clear change: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tarReader := tar.NewReader(r.Body())

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			http.Error(w, "read tar: "+err.Error(), http.StatusBadRequest)
			return
		}
		pfiBytes, err := base64.RawURLEncoding.DecodeString(header.Name)
		if err != nil {
			http.Error(w, "decode push file info: "+err.Error(), http.StatusBadRequest)
			return
		}
		var pfi protos.PushFileInfo
		if err := proto.Unmarshal(pfiBytes, &pfi); err != nil {
			http.Error(w, "unmarshal push file info: "+err.Error(), http.StatusBadRequest)
			return
		}

		if pfi.ContainsContent {
			if err := repo.SetFileContent(pfi.ContentHash, tarReader); err != nil {
				http.Error(w, "set file content: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		inConflict, err := isInConflict(repo, pfi.Name, pfi.ContentHash)
		if err != nil {
			http.Error(w, "check if file is in conflict: "+err.Error(), http.StatusInternalServerError)
			return
		}

		fileId, err := db.UpsertFile(
			tx,
			r.Context(),
			pfi.Name,
			pfi.Executable,
			pfi.ContentHash,
			inConflict,
		)
		if err != nil {
			http.Error(w, "upsert file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tx.AddFileToChange(r.Context(), changeId, fileId); err != nil {
			http.Error(w, "add file to change: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleCheckFilesExists(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	req := new(protos.CheckFilesExistsRequest)
	err = protos.Unmarshal(r.Body(), req)
	if err != nil {
		http.Error(w, "unmarshal check file exists request: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp := new(protos.CheckFilesExistsResponse)

	for _, hash := range req.ContentHash {
		if exists, err := repo.FileExists(hash); err != nil {
			http.Error(w, "check file exists: "+err.Error(), http.StatusInternalServerError)
			return
		} else {
			resp.Exists = append(resp.Exists, exists)
		}
	}

	_ = protos.MarshalWrite(resp, w)
}

func (a *App) handleNewChange(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	newChangeRequest := new(protos.NewChangeRequest)
	err = protos.Unmarshal(r.Body(), newChangeRequest)
	if err != nil {
		http.Error(w, "unmarshal new change request: "+err.Error(), http.StatusBadRequest)
		return
	}

	tx, err := db.Q.Begin(r.Context())
	if err != nil {
		http.Error(w, "begin transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Close()

	changeName, err := tx.GenerateChangeName(r.Context(), repo.ID())
	if err != nil {
		http.Error(w, "generate change name: "+err.Error(), http.StatusInternalServerError)
		return
	}

	changeId, err := tx.CreateChange(
		r.Context(),
		repo.ID(),
		changeName,
		newChangeRequest.Description,
		r.Username(),
		r.MachineID(),
		0, // overwrite depth later
	)
	if err != nil {
		http.Error(w, "create change: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, bookmark := range newChangeRequest.GetSetBookmarks() {
		if err := tx.SetBookmark(r.Context(), repo.ID(), bookmark, changeId); err != nil {
			http.Error(w, "set bookmark: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	mergeParents := make([]mergeParent, len(newChangeRequest.Parents))
	var changeDepth int64

	// set parents
	for i, parent := range newChangeRequest.Parents {
		parentChangeId, err := tx.FindChange(r.Context(), repo.ID(), parent)
		if err != nil {
			http.Error(w, "get parent change: "+err.Error(), http.StatusInternalServerError)
			return
		}
		parentChangeName, err := tx.GetChangeName(r.Context(), parentChangeId, repo.ID())
		if err != nil {
			http.Error(w, "get parent change name: "+err.Error(), http.StatusInternalServerError)
			return
		}
		mergeParents[i] = mergeParent{
			changeID:   parentChangeId,
			changeName: parentChangeName,
		}
		if err = tx.SetChangeParent(r.Context(), changeId, &parentChangeId); err != nil {
			http.Error(w, "set parent change: "+err.Error(), http.StatusInternalServerError)
			return
		}
		parentDepth, err := tx.GetChangeDepth(r.Context(), parentChangeId, repo.ID())
		if err != nil {
			http.Error(w, "get parent change depth: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if parentDepth > changeDepth {
			changeDepth = parentDepth
		}
	}
	changeDepth++

	if err = tx.SetChangeDepth(r.Context(), changeId, changeDepth); err != nil {
		http.Error(w, "set change depth: "+err.Error(), http.StatusInternalServerError)
		return
	}

	switch len(mergeParents) {
	case 0:
		http.Error(w, "new change must have at least one parent", http.StatusBadRequest)
		return
	case 1:
		// simple case, just copy all files from parent
		if err = tx.CopyFileList(r.Context(), changeId, mergeParents[0].changeID); err != nil {
			http.Error(w, "copy file list: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if same, err := tx.CheckIfChangesSameFileCount(r.Context(), changeId, mergeParents[0].changeID); err != nil {
			http.Error(w, "check if changes have same file count: "+err.Error(), http.StatusInternalServerError)
			return
		} else if !same {
			http.Error(w, "new change must have the same file count as its parent", http.StatusBadRequest)
			return
		}
	case 2:
		// more complex case, merge algorithm is required
		if err := a.Merge(r.Context(), tx, repo, changeId, mergeParents); err != nil {
			http.Error(w, "merge: "+err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "new change must have one or two parents", http.StatusBadRequest)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	ncResp := &protos.NewChangeResponse{
		ChangeId: changeId,
	}
	_ = protos.MarshalWrite(ncResp, w)
}

func (a *App) handleCheckout(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	checkoutReq := new(protos.CheckoutRequest)
	err = protos.Unmarshal(r.Body(), checkoutReq)
	if err != nil {
		http.Error(w, "unmarshal checkout request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// send the whole change via one tar stream
	files, err := db.Q.ListChangeFiles(r.Context(), checkoutReq.ChangeId)
	if err != nil {
		http.Error(w, "list change files: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tarWriter := tar.NewWriter(w)
	defer tarWriter.Close()
	for _, fileInfo := range files {
		header := &tar.Header{
			Name:     fileInfo.Name,
			Typeflag: tar.TypeReg,
		}
		if fileInfo.Executable {
			header.Mode = 0755
		} else {
			header.Mode = 0644
		}
		stat, err := repo.GetFileInfo(fileInfo.ContentHash)
		if err != nil {
			http.Error(w, "get file info: "+err.Error(), http.StatusInternalServerError)
			return
		}
		header.Size = stat.Size()
		if err := tarWriter.WriteHeader(header); err != nil {
			http.Error(w, "write tar header: "+err.Error(), http.StatusInternalServerError)
			return
		}

		f, err := repo.GetFileContent(fileInfo.ContentHash)
		if err != nil {
			http.Error(w, "get file content: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		if _, err := io.Copy(tarWriter, f); err != nil {
			http.Error(w, "copy file content: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_ = f.Close()
	}
}

func (a *App) handleSetBookmark(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	req := new(protos.SetBookmarkRequest)
	err = protos.Unmarshal(r.Body(), req)
	if err != nil {
		http.Error(w, "unmarshal set bookmark request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err = db.Q.SetBookmark(r.Context(), repo.ID(), req.Bookmark, req.ChangeId); err != nil {
		http.Error(w, "set bookmark: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (a *App) handleLog(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	req := new(protos.LogRequest)
	err = protos.Unmarshal(r.Body(), req)
	if err != nil {
		http.Error(w, "unmarshal log request: "+err.Error(), http.StatusBadRequest)
		return
	}

	head, err := db.Q.FindChange(r.Context(), repo.ID(), req.Head)
	if err != nil {
		http.Error(w, "find change: "+err.Error(), http.StatusInternalServerError)
		return
	}

	sb := &strings.Builder{}

	var tz *time.Location
	if req.TimeZone == "" {
		tz = time.UTC
	} else {
		tz, err = time.LoadLocation(req.TimeZone)
		if err != nil {
			http.Error(w, "load time zone: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	err = repo.PrintLog(repos.LogOptions{
		r.Context(),
		sb,
		req.Limit,
		tz,
		head,
	})
	if err != nil {
		http.Error(w, "print log: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = protos.MarshalWrite(&protos.LogResponse{Log: sb.String()}, w)
}

func (a *App) handleConflicts(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	req := new(protos.ConflictsRequest)
	err = protos.Unmarshal(r.Body(), req)
	if err != nil {
		http.Error(w, "unmarshal conflicts request: "+err.Error(), http.StatusBadRequest)
		return
	}

	changeId, err := db.Q.FindChange(r.Context(), repo.ID(), req.Change)
	if err != nil {
		http.Error(w, "find change: "+err.Error(), http.StatusInternalServerError)
		return
	}

	conflicts, err := db.Q.GetChangeConflicts(r.Context(), changeId)
	if err != nil {
		http.Error(w, "get change conflicts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_ = protos.MarshalWrite(&protos.ConflictsResponse{Conflicts: conflicts}, w)
}

func (a *App) handleFindChange(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	req := new(protos.FindChangeRequest)
	err = protos.Unmarshal(r.Body(), req)
	if err != nil {
		http.Error(w, "unmarshal find change request: "+err.Error(), http.StatusBadRequest)
		return
	}

	changeId, err := db.Q.FindChange(r.Context(), repo.ID(), req.Name)
	if err != nil {
		http.Error(w, "find change: "+err.Error(), http.StatusNotFound)
		return
	}

	var desc *string
	if req.IncludeDescription {
		desc, _ = db.Q.GetChangeDescription(r.Context(), changeId)
	}

	_ = protos.MarshalWrite(&protos.FindChangeResponse{ChangeId: changeId, Description: desc}, w)
}

func (a *App) handleDescribe(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	req := new(protos.DescribeRequest)
	err = protos.Unmarshal(r.Body(), req)
	if err != nil {
		http.Error(w, "unmarshal describe request: "+err.Error(), http.StatusBadRequest)
		return
	}

	changeId, err := db.Q.FindChange(r.Context(), repo.ID(), req.Change)
	if err != nil {
		http.Error(w, fmt.Sprintf("find change %s: %s", req.Change, err.Error()), http.StatusInternalServerError)
		return
	}

	if err = db.Q.SetChangeDescription(
		r.Context(),
		changeId,
		utils.Ptr(req.Description),
		r.Username(),
		r.MachineID(),
	); err != nil {
		http.Error(w, "set change description: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *App) handleListBookmarks(w http.ResponseWriter, r *signedhttp.Request) {
	repo, err := a.openRepo(r.PathValue("repo"))
	if err != nil {
		http.Error(w, "open repository: "+err.Error(), http.StatusInternalServerError)
		return
	}

	dbBookmarks, err := db.Q.GetAllBookmarks(r.Context(), repo.ID())
	if err != nil {
		http.Error(w, "list bookmarks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := new(protos.ListBookmarksResponse)

	for _, dbBookmark := range dbBookmarks {
		bookmark := &protos.Bookmark{
			BookmarkName: dbBookmark.Name,
			ChangeID:     dbBookmark.ChangeID,
		}

		prefix, err := db.Q.GetChangePrefix(r.Context(), dbBookmark.ChangeID, repo.ID())
		if err != nil {
			http.Error(w, "find change: "+err.Error(), http.StatusInternalServerError)
			return
		}
		bookmark.ChangePrefix = prefix

		changeName, err := db.Q.GetChangeName(r.Context(), dbBookmark.ChangeID, repo.ID())
		if err != nil {
			http.Error(w, "find change: "+err.Error(), http.StatusInternalServerError)
			return
		}
		bookmark.ChangeName = changeName

		resp.Bookmarks = append(resp.Bookmarks, bookmark)
	}

	_ = protos.MarshalWrite(resp, w)
}
