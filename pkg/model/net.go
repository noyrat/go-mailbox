package model

type Res struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}
type Page[T any] struct {
	List  []T `json:"list"`
	Total int `json:"total"`
}
type LoginRequest struct {
	Username string `json:"u"`
	Password string `json:"p"`
}
type ConfigAuthRequest struct {
	Username    string `json:"u"`
	Password    string `json:"p"`
	NewPassword string `json:"n"`
}
type OAuthRequest struct {
	Code  string `form:"code"`
	State string `form:"state"`
}
type ConfigOAuthRequest struct {
	GoogleClientID      string `json:"gid"`
	GoogleClientSecret  string `json:"gs"`
	GoogleRedirectUri   string `json:"gu"`
	OutlookClientID     string `json:"oid"`
	OutlookClientSecret string `json:"os"`
	OutlookRedirectUri  string `json:"ou"`
}
type MailPageRequest struct {
	AccountId   int      `json:"i"`
	BoxName     string   `json:"n"`
	PageSize    int      `json:"ps"`
	PageNum     int      `json:"pn"`
	Query       string   `json:"q"`
	QueryFields []string `json:"qf"`
	DateStart   string   `json:"ds"`
	DateEnd     string   `json:"de"`
}

type MailRequest struct {
	AccountId int    `json:"i"`
	BoxName   string `json:"n"`
	Uid       int    `json:"u"`
}

type MailAccountSwitchRequest struct {
	AccountId int `json:"i"`
	Status    int `json:"s"`
}
