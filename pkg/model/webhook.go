package model

type Webhook struct {
	Id     int             `storm:"id,increment" json:"i" gorm:"primaryKey,autoIncrement"`
	Name   string          `json:"n"`
	Url    string          `json:"u"`
	Method string          `json:"m"`
	Filter []WebhookFilter `json:"f" gorm:"serializer:json"`
	Header string          `json:"h"`
	Body   string          `json:"b"`
}
type WebhookFilter struct {
	Variable string `json:"v"`
	Match    string `json:"m"`
	Param    string `json:"p"`
}
