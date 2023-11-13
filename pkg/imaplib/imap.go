package imaplib

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mailbox/pkg/model"
	"mailbox/pkg/oauthlib"
	"mailbox/pkg/sqlitedb"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
	"github.com/jellydator/ttlcache/v3"
	"go.uber.org/zap"
	"jaytaylor.com/html2text"
)

var logger, _ = zap.NewDevelopment()

type IMAPPool struct {
	op       *sync.Mutex
	webhooks []model.Webhook
	pool     map[string]*IMAP
	seqCache *ttlcache.Cache[string, map[string][]uint32] //account+mailbox,searchmd5
}

var Pool = &IMAPPool{
	op:       &sync.Mutex{},
	pool:     make(map[string]*IMAP),
	webhooks: make([]model.Webhook, 0),
}

type IMAP struct {
	email string
	param *model.MailAccount

	spamName  string
	inboxName string

	idle  *IMAPClient
	fetch *IMAPClient

	supportSort bool
}
type IMAPClient struct {
	stop chan struct{}
	c    *client.Client
	lock sync.Mutex
}

func NewIMAP(a *model.MailAccount) *IMAP {
	return &IMAP{
		email:     a.Email,
		param:     a,
		inboxName: "",
		spamName:  "",
		idle: &IMAPClient{
			stop: make(chan struct{}),
			lock: sync.Mutex{},
		},
		fetch: &IMAPClient{
			stop: make(chan struct{}),
			lock: sync.Mutex{},
		},
	}
}

func Status() map[string]any {
	return map[string]any{}
}
func CheckLogin(m *model.MailAccount) (*client.Client, error) {
	c, err := client.DialTLS(fmt.Sprintf("%s:%d", m.Host, m.Port), nil)
	imap.CharsetReader = charset.Reader
	if err != nil {
		return nil, err
	}
	if err := c.Login(m.Account, m.Passwd); err != nil {
		return nil, err
	}
	return c, nil
}
func CheckOauthLogin(m *model.MailAccount) (c *client.Client, err error) {

	failCount := 0
	oauthHandler := oauthlib.NewHandler(m.Email)
	if oauthHandler == nil {
		return nil, errors.New("oauth site error")
	}
	if m.AccessExpire < time.Now().Unix() {
		err = errors.New("access expired")
		failCount = failCount - 1
	}
	for failCount < 3 {
		c, err = client.DialTLS(fmt.Sprintf("%s:%d", m.Host, m.Port), nil)
		if err != nil {
			logger.Warn(fmt.Sprintf("%s: dial err: %s", m.Email, err.Error()))
			return nil, err
		}
		auth := NewXoauth2Client(m.Email, m.AccessToken)
		if err = c.Authenticate(auth); err != nil {
			logger.Warn(fmt.Sprintf("%s: login err: %s", m.Email, err.Error()))
		} else {
			break
		}
		if err != nil {
			err := oauthHandler.RefreshAccount(m)
			if err != nil {
				if err.Error() == "verify failed" {
					return nil, err
				}
				failCount = failCount + 1
				continue
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if m.Id > 0 {
		sqlitedb.DB.UpMailAccount(*m)
	}
	return c, nil
}
func CheckWebhook(h model.Webhook, headers map[string]string) error {
	if req, err := http.NewRequest(h.Method, h.Url, bytes.NewBufferString(h.Body)); err != nil {
		return err
	} else {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		client := http.Client{}
		if res, err := client.Do(req); err != nil {
			return err
		} else if res.StatusCode != 200 {
			return errors.New("http code error")
		}
	}
	return nil
}
func Init() {
	Pool.seqCache = ttlcache.New[string, map[string][]uint32]()
	WithOpLock(func() {
		wp := sync.WaitGroup{}
		for _, a := range sqlitedb.DB.MailAccountList(1) {
			a := a
			imap := NewIMAP(&a)
			Pool.pool[a.Email] = imap
			wp.Add(1)
			go func() {
				imap.StartIdle(false)
				wp.Done()
			}()
		}
		wp.Wait()
		Pool.webhooks = sqlitedb.DB.WebhookList()
	})
	go Clean()
}
func StartMailAccount(a model.MailAccount) {
	WithOpLock(func() {
		if _, ok := Pool.pool[a.Email]; ok {
			return
		}
		imap := NewIMAP(&a)
		Pool.pool[a.Email] = imap

		imap.StartIdle(true)
	})
}

func PreSwitchMailAccount(a *model.MailAccount, status int) error {
	imap := Pool.pool[a.Email]
	if status == 1 && imap != nil {
		return errors.New("account has been actived")
	} else if status == 2 && imap == nil {
		return errors.New("account has been deactived")
	}
	return nil
}
func SwitchMailAccount(a *model.MailAccount, status int) error {
	imap := Pool.pool[a.Email]
	var err error
	WithOpLock(func() {
		if status == 1 { //开启
			imap := NewIMAP(a)
			if a.Passwd != "" {
				if imap.idle.c, err = CheckLogin(a); err != nil {
					err = errors.New("account login failed: " + err.Error())
				}
			} else if a.Passwd == "" {
				if imap.idle.c, err = CheckOauthLogin(a); err != nil {
					err = errors.New("account login failed: " + err.Error())
				}
			}
			if err == nil {
				Pool.pool[a.Email] = imap
				imap.StartIdle(true)
			}
		} else if status == 2 { //关闭
			if imap != nil {
				imap.idle.lock.Lock()
				imap.fetch.lock.Lock()
				imap.idle.lock.Unlock()
				imap.fetch.lock.Unlock()
				if imap.idle.stop != nil {
					imap.idle.stop <- struct{}{}
				}
				delete(Pool.pool, a.Email)
			}
		}
		if err == nil {
			sqlitedb.DB.UpAccountStatus(a.Id, status, "")
		}
	})
	return err
}
func fetchMail(m *imap.Message) *model.Mail {
	mail := &model.Mail{
		Uid:          m.Uid,
		InternalDate: m.InternalDate.Unix(),
		Attatchment:  make(map[string]model.Attatchment),
	}
	convertEnvelope(mail, m.Envelope)
	var section imap.BodySectionName
	r := m.GetBody(&section)

	mr, err := gomail.CreateReader(r)
	if err != nil {
		logger.Warn(err.Error())
		return nil
	}
	attatchmentCount := 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			logger.Warn(err.Error())
		}

		switch h := p.Header.(type) {
		case *gomail.InlineHeader:
			b, _ := ioutil.ReadAll(p.Body)
			if t, _, _ := h.ContentType(); t == "text/plain" {
				mail.Text = string(b)
			} else if t, _, _ := h.ContentType(); t == "text/html" {
				mail.Html = string(b)
			}
		case *gomail.AttachmentHeader:
			filename, _ := h.Filename()
			t, _, _ := h.ContentType()
			b, _ := ioutil.ReadAll(p.Body)
			mail.Attatchment[strconv.Itoa(attatchmentCount)] = model.Attatchment{
				Path:  strconv.Itoa(attatchmentCount),
				Mime:  t,
				Name:  filename,
				Value: b,
			}
			attatchmentCount = attatchmentCount + 1
		}
	}
	// parse html2text
	if mail.Text == "" {
		if text, err := html2text.FromString(mail.Html, html2text.Options{}); err == nil {
			mail.Text = text
		}
	}

	return mail
}
func ImapLogin(a *model.MailAccount) (c *client.Client, err error) {
	if a.Passwd != "" {
		return CheckLogin(a)
	}
	return CheckOauthLogin(a)
}
func (m *IMAP) StartIdle(init bool) error {
	var err error
	if err := tryInit(m, m.idle); err != nil {
		SwitchMailAccount(m.param, 2)
		return err
	}
	caps, _ := m.idle.c.Capability()
	if _, ok := caps["SORT"]; ok {
		m.supportSort = true
	}
	//inboxName,spamName
	boxCh := make(chan *imap.MailboxInfo)
	go func() {
		if err = m.idle.c.List("", "*", boxCh); err != nil {
			logger.Warn(fmt.Sprintf("%s: list box err: %s", m.email, err.Error()))
		}
	}()
	boxes := scanBox(boxCh)
	for _, box := range boxes {
		if box.Name == "INBOX" {
			m.inboxName = box.Name
			continue
		}
		for _, attr := range box.Attributes {
			if attr == "\\Junk" {
				m.spamName = box.Name
				continue
			}
		}
	}
	if m.spamName == "" {
		for _, box := range boxes {
			if strings.ToLower(box.Name) == "junk" {
				m.spamName = box.Name
				break
			}
			if strings.ToLower(box.Name) == "spam" {
				m.spamName = box.Name
				break
			}
		}
	}
	m.idle.lock.Lock()
	if m.param.InboxUidNext == 0 {
		tryFetchRecent(m, m.idle, m.inboxName)
	}
	if m.param.SpamUidNext == 0 {
		tryFetchRecent(m, m.idle, m.spamName)
	}
	m.idle.lock.Unlock()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
	IDLE:
		for {
			select {
			case <-ticker.C:
				m.idle.lock.Lock()
				FetchNewest(m, m.idle, m.inboxName)
				FetchNewest(m, m.idle, m.spamName)
				m.idle.lock.Unlock()
			case <-m.idle.stop:
				logger.Warn(fmt.Sprintf("%s: idle stop", m.email))
				break IDLE
			}
		}
		logger.Info(fmt.Sprintf("%s: idle end", m.email))
	}()

	return nil
}
func scanBox(boxCh chan *imap.MailboxInfo) (ret []*imap.MailboxInfo) {
	ret = make([]*imap.MailboxInfo, 0)
	for box := range boxCh {
		ret = append(ret, box)
	}
	return ret
}

func WithOpLock(f func()) {
	Pool.op.Lock()
	f()
	Pool.op.Unlock()
}

func convertAddress(addrs []*imap.Address) (name []string, address []string) {
	if len(addrs) == 0 {
		return
	}
	for _, addr := range addrs {
		name = append(name, addr.PersonalName)
		address = append(address, fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName))
	}
	return
}
func convertEnvelope(m *model.Mail, e *imap.Envelope) {
	if e == nil {
		return
	}
	m.Date = e.Date.Unix()
	m.Subject = e.Subject
	m.FromName, m.FromAddress = convertAddress(e.From)
	m.ToName, m.ToAddress = convertAddress(e.To)
	m.ReplyToName, m.ReplyToAddress = convertAddress(e.ReplyTo)
	m.CcName, m.CcAddress = convertAddress(e.Cc)
	m.BccName, m.BccAddress = convertAddress(e.Bcc)
}
func convertMailbox(m *imap.MailboxInfo) (ret *model.MailBox) {
	if m == nil {
		return nil
	}
	return &model.MailBox{
		Attributes: m.Attributes,
		Delimiter:  m.Delimiter,
		Name:       m.Name,
	}
}
