package rpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/g-wilson/runtime"
	"github.com/g-wilson/runtime/ctxtools"
	"github.com/g-wilson/runtime/hand"
	"github.com/g-wilson/runtime/logger"
)

var userAgentTempl = "%s (runtime-rpc-client 0.1)"

type RPCClient struct {
	baseURL     string
	accessToken string
	clientName  string
	httpClient  *http.Client
}

type Options struct {
	BaseURL     string
	AccessToken string
	ClientName  string
	Transport   *http.Transport
}

func New(baseURL, accessToken, clientName string) *RPCClient {
	return &RPCClient{
		baseURL:     baseURL,
		accessToken: accessToken,
		clientName:  clientName,
		httpClient:  http.DefaultClient,
	}
}

func NewWithOptions(opts Options) *RPCClient {
	httpClient := http.DefaultClient

	if opts.Transport != nil {
		httpClient = &http.Client{
			Transport: opts.Transport,
		}
	}

	return &RPCClient{
		baseURL:     opts.BaseURL,
		accessToken: opts.AccessToken,
		clientName:  opts.ClientName,
		httpClient:  httpClient,
	}
}

func (c *RPCClient) Do(ctx context.Context, method string, reqBody interface{}, res interface{}) (err error) {
	err = c.doInternal(ctx, method, reqBody, res)
	if err == nil {
		return nil
	}
	if handErr, ok := err.(hand.E); ok {
		return handErr
	}

	parsedErr := fmt.Errorf("rpc client: %w", fmt.Errorf("url=%s method=%s err=%w", c.baseURL, method, err))

	logger.FromContext(ctx).Entry().WithError(parsedErr).Warn("downstream rpc request failed")

	return hand.New(runtime.ErrCodeDownstream)
}

func (c *RPCClient) doInternal(ctx context.Context, method string, reqBody interface{}, res interface{}) (err error) {
	var req *http.Request

	if reqBody != nil {
		reqBytes, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}

		req, err = http.NewRequestWithContext(ctx, "POST", c.baseURL+"/"+method, strings.NewReader(string(reqBytes)))
		if err != nil {
			return err
		}

		req.Header.Add("Content-Type", "application/json; charset=utf-8")
	} else {
		req, err = http.NewRequestWithContext(ctx, "POST", c.baseURL+"/"+method, nil)
		if err != nil {
			return err
		}
	}

	req.Header.Add("User-Agent", fmt.Sprintf(userAgentTempl, c.clientName))
	req.Header.Add("Accept", "application/json")

	if c.accessToken != "" {
		req.Header.Add("Authorization", c.accessToken)
	}

	requestID := ctxtools.GetRequestID(ctx)
	if requestID != "" {
		req.Header.Add("X-Parent-Request-ID", requestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	resBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNoContent {
		if len(resBytes) > 0 {
			return errors.New("unexpected content for 204 response")
		}

		return nil
	}

	if resp.StatusCode == http.StatusOK {
		if len(resBytes) == 0 {
			return errors.New("no body for 200 response")
		}

		return json.Unmarshal(resBytes, res)
	}

	// parseable hand error response
	var errBody hand.E
	err = json.Unmarshal(resBytes, &errBody)
	if err == nil && errBody.Code != "" {
		return errBody
	}

	return errors.New(resp.Status)
}
