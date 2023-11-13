package imaplib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mailbox/pkg/model"
	"mailbox/pkg/utils"
	"net/http"
	"regexp"
	"strings"
)

func AddWebhook(h model.Webhook) {
	WithOpLock(func() {
		Pool.webhooks = append(Pool.webhooks, h)
	})
}
func DelWebhook(h model.Webhook) {
	WithOpLock(func() {
		Pool.webhooks = utils.Filter[model.Webhook](Pool.webhooks, func(w model.Webhook) bool {
			return w.Id == h.Id
		})
	})
}
func callWebhooks(m *IMAP, mail *model.Mail, webhooks []model.Webhook) {
	macroMap := map[string]string{}
	macroMap["{token}"] = mail.Token
	macroMap["{email}"] = m.email
	if len(mail.FromAddress) > 0 {
		macroMap["{from}"] = mail.FromAddress[0]
	}
	if len(mail.ToAddress) > 0 {
		macroMap["{to}"] = mail.ToAddress[0]
	}
	macroMap["{subject}"] = mail.Subject
	if mail.Text != "" {
		macroMap["{text}"] = mail.Text
		if len(mail.Text) > 400 {
			macroMap["{text}"] = mail.Text[0:400]
		}
	}
	if mail.Html != "" {
		macroMap["{html}"] = mail.Html
	}
	// todo  "Mailbox"
webhookFor:
	for _, w := range webhooks {
		pass := true
		for _, f := range w.Filter {
			variable := ""
			key := fmt.Sprintf("{%s}", strings.ToLower(f.Variable))
			if v, ok := macroMap[key]; ok {
				variable = v
			} else {
				continue webhookFor
			}
			if len(variable) > 0 {
				if f.Match == "=" {
					pass = pass && variable == f.Param
				} else if f.Match == "Include" {
					pass = pass && strings.Contains(variable, f.Param)
				} else if f.Match == "Exclude" {
					pass = pass && !strings.Contains(variable, f.Param)
				} else if f.Match == "RegExp" {
					match, _ := regexp.MatchString(f.Param, variable)
					pass = pass && match
				}
			} else {
				continue webhookFor
			}
			if !pass {
				continue webhookFor
			}
		}
		//todo logs
		for k, v := range macroMap {
			w.Body = strings.ReplaceAll(w.Body, k, v)
			w.Url = strings.ReplaceAll(w.Url, k, v)
		}
		req, _ := http.NewRequest(w.Method, w.Url, bytes.NewBufferString(w.Body))
		var headers map[string]string
		json.Unmarshal([]byte(w.Header), &headers)
		for hk, hv := range headers {
			for k, v := range macroMap {
				hv = strings.ReplaceAll(hv, k, v)
			}
			req.Header.Set(hk, hv)
		}
		req.Header.Set("User-Agent", "Go")
		client := http.Client{}
		if res, err := client.Do(req); err != nil {
			logger.Warn(fmt.Sprintf("webhook error: %s", err.Error()))
		} else if res.StatusCode != 200 {
			logger.Warn(fmt.Sprintf("webhook error: http code error"))
		}
	}
}
