package collector

import (
  "errors"
  "fmt"
  "io/ioutil"
  "net/http"
  "time"
)

const TOKEN_ENDPOINT = "http://169.254.169.254/latest/api/token"

// Tokenizer error
type tokenError struct {
  StatusCode int
  Err        error
}

func (r *tokenError) Error() string {
  return fmt.Sprintf("Couldn't acquire token (%d): %v", r.StatusCode, r.Err)
}

// Tokenizer transport
type tokenTransport struct {
  base    http.RoundTripper
  headers map[string]string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
  for k, v := range t.headers {
    req.Header.Add(k, v)
  }

  base := t.base
  if base == nil {
    base = http.DefaultTransport
  }

  return base.RoundTrip(req)
}

// New tokenized HTTP client
func newTokenizedHTTPClient() (*http.Client, error) {
  client := http.Client{
    Timeout: time.Duration(5 * time.Second),
  }

  req, err := http.NewRequest(http.MethodPut, TOKEN_ENDPOINT, nil)
  if err != nil {
    return nil, err
  }

  req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "15")

  resp, err := client.Do(req)
  if err != nil {
    return nil, err
  }

  defer resp.Body.Close()
  body, _ := ioutil.ReadAll(resp.Body)

  if resp.StatusCode != 200 {
    return nil, &tokenError{StatusCode: resp.StatusCode, Err: errors.New(string(body))}
  }

  return &http.Client{
    Transport: &tokenTransport{
      headers: map[string]string{
        "X-aws-ec2-metadata-token": string(body),
      },
    },
  }, nil
}
