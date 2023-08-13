package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type FirebaseAuthRestClient struct {
	apiKey     string
	projectId  string
	httpClient *http.Client
}

func NewFirebaseAuthRestClient(apiKey string, projectId string) *FirebaseAuthRestClient {
	return &FirebaseAuthRestClient{
		apiKey:    apiKey,
		projectId: projectId,
		httpClient: &http.Client{
			Timeout: time.Second * 100,
		},
	}
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("Google Identity Toolkit returned error: %v %v", e.Message, e.Code)
}

type IdTokenResponse struct {
	IdToken      string         `json:"idToken"`
	Email        string         `json:"email"`
	RefreshToken string         `json:"refreshToken"`
	ExpiresIn    string         `json:"expiresIn"`
	LocalId      string         `json:"localId"`
	Registered   bool           `json:"registered"`
	Error        *ErrorResponse `json:"error"`
}

func (f *FirebaseAuthRestClient) SignInWithEmailAndPassword(ctx context.Context, email string, password string) (IdTokenResponse, error) {
	url := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=" + f.apiKey
	body := make(map[string]string, 0)
	body["email"] = email
	body["password"] = password
	body["returnSecureToken"] = "true"
	bodyJson, err := json.Marshal(body)
	if err != nil {
		return IdTokenResponse{}, err
	}
	bodyReader := bytes.NewReader(bodyJson)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return IdTokenResponse{}, fmt.Errorf("error creating request: %w", err)
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return IdTokenResponse{}, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return IdTokenResponse{}, fmt.Errorf("error reading response: %w", err)
	}

	response := IdTokenResponse{}
	err = json.Unmarshal(respBytes, &response)
	if err != nil {
		return IdTokenResponse{}, err
	}
	return response, nil
}

type CustomTokenResponse struct {
	IdToken      string         `json:"idToken"`
	RefreshToken string         `json:"refreshToken"`
	Error        *ErrorResponse `json:"error"`
}

func (f *FirebaseAuthRestClient) SignInWithCustomToken(ctx context.Context, customToken string) (CustomTokenResponse, error) {
	url := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithCustomToken?key=" + f.apiKey
	body := make(map[string]string, 0)
	body["token"] = customToken
	body["returnSecureToken"] = "true"
	bodyJson, err := json.Marshal(body)
	if err != nil {
		return CustomTokenResponse{}, err
	}
	bodyReader := bytes.NewReader(bodyJson)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return CustomTokenResponse{}, fmt.Errorf("error creating request: %w", err)
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return CustomTokenResponse{}, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return CustomTokenResponse{}, fmt.Errorf("error reading response: %w", err)
	}

	response := CustomTokenResponse{}
	err = json.Unmarshal(respBytes, &response)
	if err != nil {
		return CustomTokenResponse{}, err
	}
	return response, nil
}
