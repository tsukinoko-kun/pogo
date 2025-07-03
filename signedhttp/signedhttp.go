package signedhttp

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tsukinoko-kun/pogo/auth"
	"github.com/tsukinoko-kun/pogo/protos"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrSignatureVerificationFailed = errors.New("signature verification failed")
	ErrInvalidSignature            = errors.New("invalid signature format")
	ErrMissingSignature            = errors.New("missing signature in trailing headers")
)

// httpSignatureData represents the data structure that gets signed for HTTP requests
type httpSignatureData struct {
	BodyHash    []byte    `json:"body_hash"`
	RequestPath string    `json:"request_path"`
	Timestamp   time.Time `json:"timestamp"`
	Username    string    `json:"username"`
	MachineID   string    `json:"machine_id"`
	PublicKey   []byte    `json:"public_key"`
}

// Client handles sending signed HTTP requests
type Client struct {
	username  string
	machineID string
	publicKey ssh.PublicKey
	client    *http.Client
}

// NewClient creates a new signed HTTP client
func NewClient(username, machineID string) (*Client, error) {
	publicKey, err := auth.GetPublicKey()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("get public key"), err)
	}

	return &Client{
		username:  username,
		machineID: machineID,
		publicKey: publicKey,
		client:    &http.Client{},
	}, nil
}

// Post sends a signed POST request with chunked transfer encoding
func (c *Client) Post(url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	// Buffer the body to calculate hash and generate signature
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
	}

	// Calculate body hash
	bodyHash := sha256.Sum256(bodyBytes)

	// Generate signature
	signature, err := c.generateSignature(url, bodyHash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to generate signature: %w", err)
	}

	// Create request with buffered body
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Force chunked transfer encoding
	req.TransferEncoding = []string{"chunked"}
	req.Header.Set("Transfer-Encoding", "chunked")

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Set up trailing headers
	req.Trailer = make(http.Header)
	req.Header.Set("Trailer", "X-Signature")
	req.Trailer.Set("X-Signature", signature)

	return c.client.Do(req)
}

func (c *Client) generateSignature(requestURL string, bodyHash []byte) (string, error) {
	now := time.Now().UTC()
	publicKeyBytes := c.publicKey.Marshal()

	// Extract path from URL
	reqURL, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	sigData := httpSignatureData{
		bodyHash,
		reqURL.URL.Path,
		now,
		c.username,
		c.machineID,
		publicKeyBytes,
	}

	dataBytes, err := json.Marshal(sigData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal signature data: %w", err)
	}

	signature, err := auth.Sign(c.publicKey, dataBytes)
	if err != nil {
		return "", fmt.Errorf("failed to sign data: %w", err)
	}

	httpSig := &protos.HTTPSignature{
		BodyHash:    sigData.BodyHash,
		RequestPath: sigData.RequestPath,
		Timestamp:   timestamppb.New(sigData.Timestamp),
		Username:    sigData.Username,
		MachineId:   sigData.MachineID,
		PublicKey:   publicKeyBytes,
		Format:      signature.Format,
		Blob:        signature.Blob,
		Rest:        signature.Rest,
	}

	sigBytes, err := proto.Marshal(httpSig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal signature: %w", err)
	}

	return base64.URLEncoding.EncodeToString(sigBytes), nil
}

// Request wraps an HTTP request for signature verification
type Request struct {
	*http.Request
	tempFile *os.File
	bodyHash []byte
	verified bool
	httpSig  *protos.HTTPSignature
}

// NewRequest creates a new signed request wrapper and verifies the signature
func NewRequest(req *http.Request) (*Request, error) {
	tempFile, err := os.CreateTemp("", "signedhttp-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	// Copy body to temp file while calculating hash
	_, err = io.Copy(multiWriter, req.Body)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to copy body: %w", err)
	}

	bodyHash := hasher.Sum(nil)

	// Reset temp file position to beginning
	_, err = tempFile.Seek(0, 0)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	signedReq := &Request{
		Request:  req,
		tempFile: tempFile,
		bodyHash: bodyHash,
	}

	// Verify signature
	err = signedReq.verifySignature()
	if err != nil {
		signedReq.Close()
		return nil, err
	}

	signedReq.verified = true
	return signedReq, nil
}

func (r *Request) verifySignature() error {
	sigHeader := r.Request.Trailer.Get("X-Signature")
	if sigHeader == "" {
		return ErrMissingSignature
	}

	sigBytes, err := base64.URLEncoding.DecodeString(sigHeader)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	var httpSig protos.HTTPSignature
	err = proto.Unmarshal(sigBytes, &httpSig)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	// Store for later access
	r.httpSig = &httpSig

	sigTime := httpSig.Timestamp.AsTime()

	now := time.Now().UTC()
	if now.Sub(sigTime) > 5*time.Minute || sigTime.Sub(now) > 5*time.Minute {
		return fmt.Errorf("%w: timestamp too old or in future", ErrSignatureVerificationFailed)
	}

	// Verify body hash matches
	if !bytes.Equal(r.bodyHash, httpSig.BodyHash) {
		return fmt.Errorf("%w: body hash mismatch", ErrSignatureVerificationFailed)
	}

	// Verify request path matches
	if r.Request.URL.Path != httpSig.RequestPath {
		return fmt.Errorf("%w: request path mismatch", ErrSignatureVerificationFailed)
	}

	// Parse public key
	publicKey, err := ssh.ParsePublicKey(httpSig.PublicKey)
	if err != nil {
		return fmt.Errorf("%w: invalid public key", ErrInvalidSignature)
	}

	// Reconstruct signature data
	sigData := httpSignatureData{
		httpSig.BodyHash,
		httpSig.RequestPath,
		sigTime,
		httpSig.Username,
		httpSig.MachineId,
		httpSig.PublicKey,
	}

	dataBytes, err := json.Marshal(sigData)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal signature data", ErrInvalidSignature)
	}

	// Verify signature
	signature := &ssh.Signature{
		Format: httpSig.Format,
		Blob:   httpSig.Blob,
		Rest:   httpSig.Rest,
	}

	err = publicKey.Verify(dataBytes, signature)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSignatureVerificationFailed, err)
	}

	return nil
}

// Body returns the request body as an io.Reader from the temp file
func (r *Request) Body() io.Reader {
	return r.tempFile
}

// Username returns the username from the verified signature
func (r *Request) Username() string {
	if !r.verified || r.httpSig == nil {
		return ""
	}
	return r.httpSig.Username
}

// MachineID returns the machine ID from the verified signature
func (r *Request) MachineID() string {
	if !r.verified || r.httpSig == nil {
		return ""
	}
	return r.httpSig.MachineId
}

// Close closes the temp file and removes it
func (r *Request) Close() error {
	if r.tempFile != nil {
		r.tempFile.Close()
		os.Remove(r.tempFile.Name())
		r.tempFile = nil
	}
	return nil
}
