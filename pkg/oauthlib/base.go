package oauthlib

import (
	"mailbox/pkg/model"
	"strings"

	"go.uber.org/zap"
)

var logger, _ = zap.NewDevelopment()

type IHandler interface {
	GetAccount(code string) (*model.MailAccount, error)
	RefreshAccount(account *model.MailAccount) error
}

func NewHandler(mail string) IHandler {
	if strings.HasSuffix(mail, "@gmail.com") {
		return &GoogleHandler{}
	} else if strings.HasSuffix(mail, "@outlook.com") {
		return &OutlookHandler{}
	}
	return nil
}
