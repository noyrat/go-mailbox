package oauthlib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mailbox/pkg/model"
	"mailbox/pkg/sqlitedb"
	"net/http"
	"net/url"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type GoogleHandler struct {
	IHandler
}

func (h *GoogleHandler) GetAccount(code string) (*model.MailAccount, error) {
	config := sqlitedb.DB.GetConfig()
	if config.GoogleClientID == "" || config.GoogleClientSecret == "" || config.GoogleRedirectUri == "" {
		return nil, errors.New("google params missing")
	}
	body := fmt.Sprintf("code=%s&client_id=%s&client_secret=%s&redirect_uri=%s&grant_type=authorization_code",
		url.QueryEscape(code),
		url.QueryEscape(config.GoogleClientID),
		url.QueryEscape(config.GoogleClientSecret),
		url.QueryEscape(config.GoogleRedirectUri),
	)
	req, _ := http.NewRequest("POST", "https://www.googleapis.com/oauth2/v4/token", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := http.Client{}
	if res, err := client.Do(req); err != nil {
		return nil, errors.New("request failed")
	} else if res.StatusCode != 200 {
		return nil, errors.New("verify failed")
	} else {
		var result map[string]interface{}
		by, _ := io.ReadAll(res.Body)
		json.Unmarshal(by, &result)
		idToken := result["id_token"].(string)
		accessToken := result["access_token"].(string)
		refreshToken := result["refresh_token"].(string)
		expires := int64(result["expires_in"].(float64))

		token, _, err := new(jwt.Parser).ParseUnverified(idToken, jwt.MapClaims{})
		if err != nil {
			return nil, errors.New("decode failed")
		}
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			email := claims["email"].(string)
			account := &model.MailAccount{
				Email:   email,
				Account: email,
				Host:    "imap.gmail.com",
				Port:    993,
				Type:    "oauth",

				RefreshToken: refreshToken,
				AccessToken:  accessToken,
				AccessExpire: time.Now().Unix() + expires,
			}
			return account, nil
		}
		return nil, errors.New("decode failed")
	}
}
func (h *GoogleHandler) RefreshAccount(account *model.MailAccount) error {
	config := sqlitedb.DB.GetConfig()
	if config.GoogleClientID == "" || config.GoogleClientSecret == "" || config.GoogleRedirectUri == "" {
		return errors.New("google params missing")
	}
	body := fmt.Sprintf("refresh_token=%s&client_id=%s&client_secret=%s&redirect_uri=%s&grant_type=refresh_token",
		url.QueryEscape(account.RefreshToken),
		url.QueryEscape(config.GoogleClientID),
		url.QueryEscape(config.GoogleClientSecret),
		url.QueryEscape(config.GoogleRedirectUri),
	)
	req, _ := http.NewRequest("POST", "https://www.googleapis.com/oauth2/v4/token", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := http.Client{}
	if res, err := client.Do(req); err != nil {
		return errors.New("request failed")
	} else if res.StatusCode != 200 {
		sqlitedb.DB.UpAccountStatus(account.Id, 2, "refresh code expired")
		return errors.New(fmt.Sprintf("verify failed"))
	} else {
		var result map[string]interface{}
		by, _ := io.ReadAll(res.Body)
		json.Unmarshal(by, &result)
		accessToken := result["access_token"].(string)
		expires := int64(result["expires_in"].(float64))
		account.AccessToken = accessToken
		account.AccessExpire = time.Now().Unix() + expires
		return nil
	}
}
