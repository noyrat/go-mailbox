package model

type Config struct {
	Id     int    `storm:"id" json:"i" gorm:"primaryKey"`
	User   string `json:"u"`
	Passwd string `json:"p"`
	Salt   string `json:"s"`

	GoogleClientID     string `json:"gid"`
	GoogleClientSecret string `json:"gs"`
	GoogleRedirectUri  string `json:"gu"`

	OutlookClientID     string `json:"oid"`
	OutlookClientSecret string `json:"os"`
	OutlookRedirectUri  string `json:"ou"`
}
