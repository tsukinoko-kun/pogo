package serve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"strings"

	"github.com/tsukinoko-kun/pogo/db"
	"github.com/tsukinoko-kun/pogo/repos"
	"github.com/tsukinoko-kun/pogo/text"
	"github.com/tsukinoko-kun/pogo/utils"

	"github.com/devsisters/go-diff3"
)

type mergeParent struct {
	changeID   int64
	changeName string
}

func (a *App) Merge(ctx context.Context, q db.Querier, repo repos.Repo, targetChangeId int64, mergeParents []mergeParent) error {
	var lca int64
	{
		ancestries := make([][]db.GetAncestryOfChangeRow, len(mergeParents))
		for i, head := range mergeParents {
			ancestry, err := q.GetAncestryOfChange(ctx, head.changeID, 100, repo.ID())
			if err != nil {
				return errors.Join(errors.New("get ancestry of change"), err)
			}
			ancestries[i] = ancestry
		}
		var ok bool
		lca, ok = getLCA(ancestries)
		if !ok {
			return errors.New("no common ancestor found")
		}
	}

	mergeParentsOverlapChanges := make([]overlapChange, len(mergeParents))
	for i, mergeParent := range mergeParents {
		mergeParentsOverlapChanges[i] = overlapChange{mergeParent.changeName, mergeParent.changeID}
	}
	overlapRes, err := overlap(ctx, q, repo, overlapChange{changeID: lca, name: "LCA"}, mergeParentsOverlapChanges)
	if err != nil {
		return errors.Join(errors.New("overlap"), err)
	}

	for fileChange := range joinOverlappingChanges(overlapRes) {
		if fileChange.IsText() {
			if err := repo.SetFileContent(
				fileChange.ContentHash(),
				fileChange.TextContent().Reader(),
			); err != nil {
				return errors.Join(fmt.Errorf("set text file %s content", fileChange.FileName()), err)
			}
		}
		fileId, err := db.UpsertFile(
			q,
			ctx,
			fileChange.FileName(),
			utils.Ptr(fileChange.Executable()),
			fileChange.ContentHash(),
			fileChange.Conflict(),
		)
		if err != nil {
			return errors.Join(fmt.Errorf("upsert file %s", fileChange.FileName()), err)
		}
		if err := q.AddFileToChange(ctx, targetChangeId, fileId); err != nil {
			return errors.Join(fmt.Errorf("add file %s to change %d", fileChange.FileName(), targetChangeId), err)
		}
	}

	return nil
}

// getLCA calculates the lowest common ancestor of a set of commits.
// It returns the change ID and a boolean indicating whether the LCA was found.
func getLCA(ancestries [][]db.GetAncestryOfChangeRow) (int64, bool) {
	if len(ancestries) == 0 {
		return 0, false
	}

	if len(ancestries) == 1 {
		if len(ancestries[0]) > 0 {
			return ancestries[0][0].ID, true // return the tip itself
		}
		return 0, false
	}

	// Initialize candidates with first ancestry (ID -> depth)
	candidates := make(map[int64]int64)
	for _, change := range ancestries[0] {
		candidates[change.ID] = change.Depth
	}

	// Intersect with each subsequent ancestry
	for i := 1; i < len(ancestries); i++ {
		current := make(map[int64]int64)
		for _, change := range ancestries[i] {
			if _, exists := candidates[change.ID]; exists {
				current[change.ID] = change.Depth
			}
		}
		candidates = current

		if len(candidates) == 0 {
			return 0, false // no common ancestors
		}
	}

	// Find common ancestor with maximum depth (closest to tips)
	var lcaID int64
	var maxDepth int64 = -1
	for id, depth := range candidates {
		if depth > maxDepth {
			maxDepth = depth
			lcaID = id
		}
	}

	return lcaID, true
}

type (
	overlapChange struct {
		name     string
		changeID int64
	}

	textFileChange struct {
		others         []textFileOtherChange
		base           *text.Text
		executable     bool
		baseExecutable bool
		baseExists     bool
	}

	binaryFileChange struct {
		baseHash       []byte
		otherHashes    []binaryfileOtherChange
		baseExists     bool
		executable     bool
		baseExecutable bool
	}

	binaryfileOtherChange struct {
		contentHash []byte
		changeName  string
	}

	textFileOtherChange struct {
		textContent *text.Text
		changeName  string
	}

	overlapResult struct {
		textFileChanges   map[string]*textFileChange
		binaryFileChanges map[string]*binaryFileChange
	}
)

func overlap(ctx context.Context, q db.Querier, repo repos.Repo, base overlapChange, parents []overlapChange) (overlapResult, error) {
	textFileChanges := make(map[string]*textFileChange)
	binaryFileChanges := make(map[string]*binaryFileChange)

	baseFiles, err := q.ListChangeFiles(ctx, base.changeID)
	if err != nil {
		return overlapResult{}, errors.Join(fmt.Errorf("get base files for change %s", base.name), err)
	}

	for _, baseFile := range baseFiles {
		{
			fi, err := repo.GetFileInfo(baseFile.ContentHash)
			if err != nil {
				return overlapResult{}, errors.Join(fmt.Errorf("get file info %s in change %s", baseFile.Name, base.name), err)
			}
			if fi.Size() > 1024*1024 {
				// > 1MB, treat as binary
				binaryFileChanges[baseFile.Name] = &binaryFileChange{
					baseHash:   baseFile.ContentHash,
					baseExists: true,
				}
				continue
			}
		}
		f, err := repo.GetFileContent(baseFile.ContentHash)
		if err != nil {
			return overlapResult{}, errors.Join(fmt.Errorf("get file content %s in change %s", baseFile.Name, base.name), err)
		}
		defer f.Close()
		decompFile := utils.Decompress(f)
		if f, isText, err := text.IsTextReader(decompFile); err != nil {
			return overlapResult{}, errors.Join(fmt.Errorf("detect text from file %s in change %s", baseFile.Name, base.name), err)
		} else if isText {
			txt, err := text.ReadFrom(f)
			if err != nil {
				return overlapResult{}, errors.Join(fmt.Errorf("read text from file %s in change %s", baseFile.Name, base.name), err)
			}
			textFileChanges[baseFile.Name] = &textFileChange{
				base:           txt,
				baseExists:     true,
				executable:     baseFile.Executable,
				baseExecutable: baseFile.Executable,
			}
		} else {
			// binary
			binaryFileChanges[baseFile.Name] = &binaryFileChange{
				baseHash:       baseFile.ContentHash,
				baseExists:     true,
				executable:     baseFile.Executable,
				baseExecutable: baseFile.Executable,
			}
		}
	}

	for _, parent := range parents {
		parentFiles, err := q.ListChangeFiles(ctx, parent.changeID)
		if err != nil {
			return overlapResult{}, errors.Join(fmt.Errorf("get parent files for change %s", parent.name), err)
		}
		for _, parentFile := range parentFiles {
			// look if base file is binary
			if change, ok := binaryFileChanges[parentFile.Name]; ok {
				change.otherHashes = append(change.otherHashes, binaryfileOtherChange{
					parentFile.ContentHash,
					parent.name,
				})
				if change.baseExecutable != parentFile.Executable {
					change.executable = parentFile.Executable
				}
				continue
			}
			// look if base file is text
			if change, ok := textFileChanges[parentFile.Name]; ok {
				f, err := repo.GetFileContent(parentFile.ContentHash)
				if err != nil {
					return overlapResult{}, errors.Join(fmt.Errorf("get file content %s in change %s", parentFile.Name, parent.name), err)
				}
				defer f.Close()
				decompFile := utils.Decompress(f)
				txt, err := text.ReadFrom(decompFile)
				if err != nil {
					return overlapResult{}, errors.Join(fmt.Errorf("read text from file %s in change %s", parentFile.Name, parent.name), err)
				}
				change.others = append(change.others, textFileOtherChange{
					textContent: txt,
					changeName:  parent.name,
				})
				if change.baseExecutable != parentFile.Executable {
					change.executable = parentFile.Executable
				}
				continue
			}
			// no base file found, add as new file
			fi, err := repo.GetFileInfo(parentFile.ContentHash)
			if err != nil {
				return overlapResult{}, errors.Join(fmt.Errorf("get file info %s in change %s", parentFile.Name, parent.name), err)
			}
			if fi.Size() > 1024*1024 {
				// > 1MB, treat as binary
				binaryFileChanges[parentFile.Name] = &binaryFileChange{
					otherHashes: []binaryfileOtherChange{{
						parentFile.ContentHash,
						parent.name,
					}},
					executable: parentFile.Executable,
					baseExists: false,
				}
				continue
			}
			f, err := repo.GetFileContent(parentFile.ContentHash)
			if err != nil {
				return overlapResult{}, errors.Join(fmt.Errorf("get file content %s in change %s", parentFile.Name, parent.name), err)
			}
			defer f.Close()
			if f, isText, err := text.IsTextReader(f); err != nil {
				return overlapResult{}, errors.Join(fmt.Errorf("detect text from file %s in change %s", parentFile.Name, parent.name), err)
			} else if isText {
				txt, err := text.ReadFrom(f)
				if err != nil {
					return overlapResult{}, errors.Join(fmt.Errorf("read text from file %s in change %s", parentFile.Name, parent.name), err)
				}
				textFileChanges[parentFile.Name] = &textFileChange{
					others:     []textFileOtherChange{{txt, parent.name}},
					base:       nil,
					baseExists: false,
					executable: parentFile.Executable,
				}
			} else {
				binaryFileChanges[parentFile.Name] = &binaryFileChange{
					otherHashes: []binaryfileOtherChange{{
						parentFile.ContentHash,
						parent.name,
					}},
					baseExists: false,
					executable: parentFile.Executable,
				}
			}
		}
	}

	return overlapResult{
		textFileChanges:   textFileChanges,
		binaryFileChanges: binaryFileChanges,
	}, nil
}

type (
	joinOverlapResult interface {
		IsText() bool
		FileName() string
		TextContent() *text.Text
		ContentHash() []byte
		Conflict() bool
		Executable() bool
	}

	joinOverlapTextResult struct {
		fileName   string
		content    *text.Text
		conflict   bool
		executable bool
	}

	joinOverlapBinaryResult struct {
		fileName   string
		hash       []byte
		conflict   bool
		executable bool
	}
)

func (r joinOverlapTextResult) IsText() bool {
	return true
}

func (r joinOverlapTextResult) FileName() string {
	return r.fileName
}

func (r joinOverlapTextResult) TextContent() *text.Text {
	return r.content
}

func (r joinOverlapTextResult) ContentHash() []byte {
	return utils.HashReader(r.content.Reader())
}

func (r joinOverlapTextResult) Conflict() bool {
	return r.conflict
}

func (r joinOverlapTextResult) Executable() bool {
	return r.executable
}

func (r joinOverlapBinaryResult) IsText() bool {
	return false
}

func (r joinOverlapBinaryResult) FileName() string {
	return r.fileName
}

func (r joinOverlapBinaryResult) TextContent() *text.Text {
	return nil
}

func (r joinOverlapBinaryResult) ContentHash() []byte {
	return r.hash
}

func (r joinOverlapBinaryResult) Conflict() bool {
	return r.conflict
}

func (r joinOverlapBinaryResult) Executable() bool {
	return r.executable
}

func joinOverlappingChanges(overlap overlapResult) iter.Seq[joinOverlapResult] {
	return func(yield func(joinOverlapResult) bool) {
		for textChange := range joinOverlappingTextChanges(overlap.textFileChanges) {
			if !yield(textChange) {
				return
			}
		}
		for binaryChange := range joinOverlappingBinaryChanges(overlap.binaryFileChanges) {
			if !yield(binaryChange) {
				return
			}
		}
	}
}

func joinOverlappingTextChanges(textFileChanges map[string]*textFileChange) iter.Seq[joinOverlapTextResult] {
	return func(yield func(joinOverlapTextResult) bool) {
		for fileName, textChange := range textFileChanges {
			if textChange.baseExists && len(textChange.others) == 0 {
				if !yield(joinOverlapTextResult{
					fileName,
					textChange.base,
					len(textChange.others) > 1,
					textChange.baseExecutable,
				}) {
					return
				}
				continue
			}
			if len(textChange.others) == 1 {
				if !yield(joinOverlapTextResult{
					fileName,
					textChange.others[0].textContent,
					false,
					textChange.executable,
				}) {
					return
				}
			} else {
				// more than one change
				if mergedContent, conflict, err := mergeTextChanges(textChange.base, textChange.others); err != nil {
					fmt.Fprintf(os.Stderr, "error merging text changes: %v\n", err)
					continue
				} else {
					if !yield(joinOverlapTextResult{
						fileName,
						mergedContent,
						conflict,
						textChange.executable,
					}) {
						return
					}
				}
			}
		}
	}
}

func joinOverlappingBinaryChanges(binaryFileChanges map[string]*binaryFileChange) iter.Seq[joinOverlapBinaryResult] {
	return func(yield func(joinOverlapBinaryResult) bool) {
		for fileName, binaryChange := range binaryFileChanges {
			if binaryChange.baseExists && len(binaryChange.otherHashes) != 1 {
				if !yield(joinOverlapBinaryResult{
					fileName,
					binaryChange.baseHash,
					false,
					binaryChange.baseExecutable,
				}) {
					return
				}
			}
			if len(binaryChange.otherHashes) == 1 {
				otherChange := binaryChange.otherHashes[0]
				if !yield(joinOverlapBinaryResult{
					fileName,
					otherChange.contentHash,
					false,
					binaryChange.executable,
				}) {
					return
				}
			} else {
				for _, otherChange := range binaryChange.otherHashes {
					if !yield(joinOverlapBinaryResult{
						fileName + ".binconflict_" + otherChange.changeName,
						otherChange.contentHash,
						true,
						binaryChange.executable,
					}) {
						return
					}
				}
			}
		}
	}
}

func mergeTextChanges(base *text.Text, others []textFileOtherChange) (*text.Text, bool, error) {
	if len(others) <= 1 {
		if len(others) == 1 {
			return others[0].textContent, false, nil
		}
		if base != nil {
			return base, false, nil
		}
		return text.NewText(""), false, nil
	}

	// Check if all changes are identical
	firstContent := others[0].textContent.String()
	allSame := true
	for i := 1; i < len(others); i++ {
		if others[i].textContent.String() != firstContent {
			allSame = false
			break
		}
	}

	if allSame {
		return others[0].textContent, false, nil
	}

	diff, err := diff3.Merge(
		others[0].textContent.Utf8Reader(),
		base.Utf8Reader(),
		others[1].textContent.Utf8Reader(),
		true,
		others[0].changeName,
		others[1].changeName,
	)
	if err != nil {
		return nil, false, errors.Join(fmt.Errorf("merge text changes"), err)
	}
	diffBytes, err := io.ReadAll(diff.Result)
	if err != nil {
		return nil, false, errors.Join(fmt.Errorf("read diff bytes"), err)
	}
	diffStr := string(diffBytes)
	return text.NewTextWithEncoding(diffStr, base.Encoding()), diff.Conflicts, nil
}

func isInConflict(repo repos.Repo, name string, contentHash []byte) (bool, error) {
	if strings.Contains(name, ".binconflict_") {
		return true, nil
	}
	f, err := repo.GetFileContent(contentHash)
	if err != nil {
		return false, errors.Join(fmt.Errorf("get file %s content", name), err)
	}
	defer f.Close()
	decompFile := utils.Decompress(f)

	if f, isText, err := text.IsTextReader(decompFile); err != nil {
		return false, errors.Join(fmt.Errorf("detect text from file %s", name), err)
	} else if !isText {
		return false, nil
	} else {
		txt, err := text.ReadFrom(f)
		if err != nil {
			return false, errors.Join(fmt.Errorf("read text from file %s", name), err)
		}
		markers := []string{"<<<<<<<<<", "=========", ">>>>>>>>>"}
		for _, marker := range markers {
			if !strings.Contains(txt.String(), marker) {
				return false, nil
			}
		}
		// all markers found
		return true, nil
	}
}
