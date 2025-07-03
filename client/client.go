package client

import (
	"archive/tar"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tsukinoko-kun/pogo/colors"
	"github.com/tsukinoko-kun/pogo/config"
	"github.com/tsukinoko-kun/pogo/protos"
	"github.com/tsukinoko-kun/pogo/serve/serveerrors"
	"github.com/tsukinoko-kun/pogo/signedhttp"
	"github.com/tsukinoko-kun/pogo/sysid"
	"github.com/tsukinoko-kun/pogo/utils"

	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type Client struct {
	url        string
	rootDir    string
	stdout     io.Writer
	headName   string
	userName   string
	machineId  string
	httpClient *signedhttp.Client
}

type RepoConfig struct {
	Host   string `yaml:"host"`
	RepoID int32  `yaml:"repo"`
}

func Init(host string, repoName string, userName string, machineId string) (*Client, error) {
	absPath, err := os.Getwd()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("get current directory"), err)
	}
	urlStr, err := url.JoinPath(host, "rpc")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("join path %s", host), err)
	}
	c := &Client{
		url:     urlStr,
		rootDir: absPath,
		stdout:  os.Stdout,
	}
	if err = c.Login(userName, machineId); err != nil {
		return nil, errors.Join(fmt.Errorf("login"), err)
	}
	repoId, err := c.Init(repoName)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("init"), err)
	}
	config := RepoConfig{
		Host:   host,
		RepoID: repoId,
	}
	f, err := os.Create(filepath.Join(absPath, ".pogo"))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create config file"), err)
	}
	defer f.Close()
	yamlEnc := yaml.NewEncoder(f)
	yamlEnc.SetIndent(4)
	err = yamlEnc.Encode(config)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("encode config file"), err)
	}

	urlStr, err = url.JoinPath(host, "rpc", fmt.Sprintf("%d", repoId))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("join path %s %d (%s)", host, repoId, repoName), err)
	}
	c.url = urlStr

	return c, nil
}

func (c *Client) Login(userName string, machineId string) error {
	c.headName = fmt.Sprintf("__head-%s-%s", userName, machineId)
	c.userName = userName
	c.machineId = machineId
	var err error
	c.httpClient, err = signedhttp.NewClient(userName, machineId)
	if err != nil {
		return errors.Join(fmt.Errorf("create http client"), err)
	}
	return nil
}

func (c *Client) Head() (string, error) {
	if c.headName == "" {
		return "", errors.New("no head set")
	}
	return c.headName, nil
}

func Open(fileName string) (*Client, error) {
	if stat, err := os.Stat(fileName); err != nil {
		return nil, err
	} else if stat.IsDir() {
		return nil, errors.New("file is a directory")
	} else if !strings.HasSuffix(fileName, ".pogo") {
		return nil, errors.New("file does not have .pogo extension")
	} else if stat.Size() == 0 || stat.Size() > 1024 {
		return nil, errors.New("file is not a pogo remote repository")
	}

	absPath, err := filepath.Abs(fileName)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("get absolute path %s", fileName), err)
	}

	f, err := os.Open(fileName)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open %s", fileName), err)
	}
	defer f.Close()

	repoConfig := RepoConfig{}
	yamlDec := yaml.NewDecoder(f)
	err = yamlDec.Decode(&repoConfig)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("decode config file"), err)
	}

	urlStr, err := url.JoinPath(repoConfig.Host, "rpc", fmt.Sprintf("%d", repoConfig.RepoID))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("join path %s %d", repoConfig.Host, repoConfig.RepoID), err)
	}

	c := &Client{
		url:     urlStr,
		rootDir: filepath.Dir(absPath),
		stdout:  os.Stdout,
	}

	userName := config.GetUsername()
	machineId, err := sysid.GetMachineID()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("get machine id"), err)
	}

	if err = c.Login(userName, machineId); err != nil {
		return nil, errors.Join(fmt.Errorf("login"), err)
	}

	return c, nil
}

func (c *Client) Init(repoName string) (int32, error) {
	headName, err := c.Head()
	if err != nil {
		return 0, errors.Join(fmt.Errorf("get head"), err)
	}

	req := &protos.InitRequest{
		Bookmarks: []string{headName, "main"},
		Name:      repoName,
	}
	resp := new(protos.InitResponse)
	if err = c.execute("init", req, resp); err != nil {
		return 0, errors.Join(fmt.Errorf("init"), err)
	}
	return resp.RepoID, nil
}

func (c *Client) Push() error {
	headName, err := c.Head()
	if err != nil {
		return errors.Join(fmt.Errorf("get head"), err)
	}

	localFiles := []struct {
		absPath        string
		name           string
		hash           []byte
		existsOnServer bool
	}{}

	// first, look what file contents are missing on the server
	cfeReq := new(protos.CheckFilesExistsRequest)
	for absPath, name := range getLocalFiles(c.rootDir) {
		hash := utils.HashFile(absPath)
		cfeReq.ContentHash = append(cfeReq.ContentHash, hash)
		localFiles = append(localFiles, struct {
			absPath        string
			name           string
			hash           []byte
			existsOnServer bool
		}{absPath, name, hash, false})
	}
	cfeRes := new(protos.CheckFilesExistsResponse)
	if err = c.execute("check_files_exists", cfeReq, cfeRes); err != nil {
		return errors.Join(errors.New("execute check_file_exists"), err)
	}

	if len(cfeRes.Exists) != len(localFiles) {
		return errors.New("check_file_exists returned unexpected number of results")
	}

	// apply the results to the localFiles
	for i := range localFiles {
		localFiles[i].existsOnServer = cfeRes.Exists[i]
	}

	// now, push push all local files
	// if the file does exist on the server, exclude the content

	pipeReader, pipeWriter := io.Pipe()
	go func() {
		defer pipeWriter.Close()
		tarWriter := tar.NewWriter(pipeWriter)
		defer tarWriter.Close()
		for _, localFile := range localFiles {
			pfi := &protos.PushFileInfo{
				Name:            localFile.name,
				Executable:      utils.IsExecutable(localFile.absPath),
				ContentHash:     localFile.hash,
				ContainsContent: !localFile.existsOnServer,
			}
			pfiBytes, _ := proto.Marshal(pfi)
			header := &tar.Header{
				Name:     base64.RawURLEncoding.EncodeToString(pfiBytes),
				Typeflag: tar.TypeReg,
			}
			if localFile.existsOnServer {
				header.Size = 0
			} else {
				header.Size, _ = utils.GetFileSize(localFile.absPath)
			}
			if err := tarWriter.WriteHeader(header); err != nil {
				panic(err)
			}
			if !localFile.existsOnServer {
				f, _ := os.Open(localFile.absPath)
				defer f.Close()
				// send uncompressed
				// compression is done on the server to make the client logic simpler
				if _, err := io.Copy(tarWriter, f); err != nil {
					panic(err)
				}
			}
		}
	}()

	rc, err := c.executeStream(
		"push",
		pipeReader,
		map[string]string{
			"X-Change-Name": headName,
		},
	)
	if err != nil {
		if strings.Contains(err.Error(), serveerrors.ErrPushToChangeWithChild.Error()) {
			fmt.Println(colors.BrightBlack + "(" + serveerrors.ErrPushToChangeWithChild.Error() + ")" + colors.Reset)
			return nil
		}
		if strings.Contains(err.Error(), serveerrors.ErrPushToChangeNotOwned.Error()) {
			fmt.Println(colors.BrightBlack + "(" + serveerrors.ErrPushToChangeNotOwned.Error() + ")" + colors.Reset)
			return nil
		}
		return errors.Join(fmt.Errorf("execute push"), err)
	}
	defer rc.Close()

	return nil
}

func (c *Client) NewChange(parents []string, description *string, setBookmarks []string) (*protos.NewChangeResponse, error) {
	resp := new(protos.NewChangeResponse)
	err := c.execute("new_change", &protos.NewChangeRequest{
		Parents:      parents,
		Description:  description,
		SetBookmarks: setBookmarks,
	}, resp)
	return resp, err
}

func (c *Client) EditName(change string) error {
	changeObj, err := c.FindChange(change, false)
	if err != nil {
		return errors.Join(fmt.Errorf("find change %s", change), err)
	}
	return c.Edit(changeObj.ChangeId)
}

func (c *Client) FindChange(change string, withDescription bool) (*protos.FindChangeResponse, error) {
	res := new(protos.FindChangeResponse)
	err := c.execute("find_change", &protos.FindChangeRequest{
		Name:               change,
		IncludeDescription: withDescription,
	}, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) Edit(changeId int64) error {
	body, err := c.executeStream("checkout", protos.Marshal(&protos.CheckoutRequest{
		ChangeId: changeId,
	}), nil)
	if err != nil {
		return errors.Join(fmt.Errorf("checkout %d", changeId), err)
	}
	defer body.Close()
	tarReader := tar.NewReader(body)
	var touchedFileNames []string

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return errors.Join(fmt.Errorf("read tar"), err)
		}
		touchedFileNames = append(touchedFileNames, header.Name)
		err = os.MkdirAll(filepath.Dir(filepath.Join(c.rootDir, header.Name)), 0755)
		if err != nil {
			return errors.Join(fmt.Errorf("mkdir %s", filepath.Dir(filepath.Join(c.rootDir, header.Name))), err)
		}
		f, err := os.Create(filepath.Join(c.rootDir, header.Name))
		if err != nil {
			return errors.Join(fmt.Errorf("create %s", header.Name), err)
		}
		defer f.Close()
		if _, err := io.Copy(f, utils.Decompress(tarReader)); err != nil {
			return errors.Join(fmt.Errorf("copy stream to %s", header.Name), err)
		}
	}

	// delete local files that are not needed anymore
	ignoreMatcher := getLocalIgnoreMatcherFiltered(c.rootDir, func(relUnixPath string) bool {
		return slices.Contains(touchedFileNames, relUnixPath)
	})
	for _, name := range getLocalFiles(c.rootDir) {
		if slices.Contains(touchedFileNames, name) || ignoreMatcher.Match(strings.Split(name, "/"), false) {
			continue
		}
		if err := os.Remove(filepath.Join(c.rootDir, name)); err != nil {
			return errors.Join(fmt.Errorf("remove %s", name), err)
		}
	}

	headName, err := c.Head()
	if err != nil {
		return errors.Join(errors.New("get head"), err)
	}

	if err = c.execute("set_bookmark", &protos.SetBookmarkRequest{
		Bookmark: headName,
		ChangeId: changeId,
	}, nil); err != nil {
		return errors.Join(fmt.Errorf("set bookmark '%s' to change %d", headName, changeId), err)
	}

	return nil
}

func (c *Client) LogLimit(limit int32) error {
	head, err := c.Head()
	if err != nil {
		return errors.Join(fmt.Errorf("get head"), err)
	}

	res := new(protos.LogResponse)
	if err = c.execute("log", &protos.LogRequest{
		Head:     head,
		Limit:    limit,
		TimeZone: time.Local.String(),
	}, res); err != nil {
		return errors.Join(fmt.Errorf("log request"), err)
	}

	fmt.Println(res.GetLog())

	return nil
}

func (c *Client) Log() error {
	head, err := c.Head()
	if err != nil {
		return errors.Join(fmt.Errorf("get head"), err)
	}

	res := new(protos.LogResponse)
	if err = c.execute("log", &protos.LogRequest{
		Head:     head,
		TimeZone: time.Local.String(),
	}, res); err != nil {
		return errors.Join(fmt.Errorf("log request"), err)
	}

	fmt.Println(res.GetLog())

	return nil
}

func (c *Client) Conflicts(change string) ([]string, error) {
	res := new(protos.ConflictsResponse)
	err := c.execute("conflicts", &protos.ConflictsRequest{
		Change: change,
	}, res)
	if err != nil {
		return nil, err
	}
	return res.Conflicts, nil
}

func (c *Client) Describe(change string, description string) error {
	return c.execute("describe", &protos.DescribeRequest{
		Change:      change,
		Description: description,
	}, nil)
}

func (c *Client) ListBookmarks() ([]*protos.Bookmark, error) {
	res := new(protos.ListBookmarksResponse)
	err := c.execute("list_bookmarks", nil, res)
	if err != nil {
		return nil, err
	}
	return res.Bookmarks, nil
}

func (c *Client) executeStream(f string, reqBody io.Reader, headers map[string]string) (io.ReadCloser, error) {
	urlStr, err := url.JoinPath(c.url, f)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Post(urlStr, reqBody, headers)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("signed http post %s", urlStr), err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if resp != nil {
			defer resp.Body.Close()
		}
		return nil, errFromResp(resp)
	}
	return resp.Body, nil
}

func (c *Client) execute(f string, reqBody proto.Message, respBody proto.Message) error {
	body, err := c.executeStream(f, protos.Marshal(reqBody), nil)
	if err != nil {
		return errors.Join(fmt.Errorf("execute stream %s", f), err)
	}
	defer body.Close()
	if respBody != nil {
		if err := protos.Unmarshal(body, respBody); err != nil {
			return errors.Join(fmt.Errorf("unmarshal response from %s", f), err)
		}
	}
	return nil
}

func errFromResp(resp *http.Response) error {
	if resp.StatusCode < http.StatusBadRequest {
		return nil
	}
	rb, _ := io.ReadAll(resp.Body)
	if len(rb) == 0 {
		return errors.New(resp.Status)
	}
	return fmt.Errorf("%s: %s", resp.Status, string(rb))
}
