package imaplib

import (
	"fmt"
	"mailbox/pkg/model"
	"mailbox/pkg/sqlitedb"
	"mailbox/pkg/utils"
	"time"

	"github.com/emersion/go-imap"
	sortthread "github.com/emersion/go-imap-sortthread"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
)

func tryInit(m *IMAP, clientP *IMAPClient) error {
	for {
		var err error
		var newClient *client.Client
		if clientP.c == nil || clientP.c.State() == imap.LogoutState {
			if newClient, err = ImapLogin(m.param); err != nil {
				if err.Error() == "verify failed" {
					return err
				}
				logger.Warn(fmt.Sprintf("%s: %s", m.email, err.Error()))
				clientP.c = nil
			} else {
				clientP.c = newClient
			}
		}
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	return nil
}
func trySelect(m *IMAP, clientP *IMAPClient, boxName string, force bool) (mailbox *imap.MailboxStatus) {
	var err error
	for {
		tryInit(m, clientP)
		if mailbox = clientP.c.Mailbox(); mailbox == nil || mailbox.Name != boxName || force {
			mailbox, err = clientP.c.Select(boxName, true)
			if err != nil {
				logger.Warn(fmt.Sprintf("%s: %s", m.email, err.Error()))
				clientP.c = nil
			}
		}
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	return
}
func trySortUidSearch(m *IMAP, clientP *IMAPClient, boxName string, search *imap.SearchCriteria) (uids []uint32, status *imap.MailboxStatus) {
	var err error
	for {
		tryInit(m, clientP)
		status = trySelect(m, clientP, boxName, true)
		sc := sortthread.NewSortClient(clientP.c)
		sortCriteria := []sortthread.SortCriterion{
			{Field: sortthread.SortDate, Reverse: false},
		}
		uids, err = sc.UidSort(sortCriteria, search)
		if err != nil {
			logger.Warn(fmt.Sprintf("%s: %s", m.email, err.Error()))
			clientP.c = nil
		}
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	return
}
func tryUidSearch(m *IMAP, clientP *IMAPClient, boxName string, search *imap.SearchCriteria) (uids []uint32, status *imap.MailboxStatus) {
	if m.supportSort {
		uids, status = trySortUidSearch(m, clientP, boxName, search)
		return
	}
	for {
		var err error
		tryInit(m, clientP)
		status = trySelect(m, clientP, boxName, true)
		imap.CharsetReader = charset.Reader
		uids, err = clientP.c.UidSearch(search)
		if err != nil {
			logger.Warn(fmt.Sprintf("%s: %s", m.email, err.Error()))
			clientP.c = nil
			continue
		}
		if len(uids) > 0 {
			seq := &imap.SeqSet{}
			seq.AddNum(uids...)
			ch := make(chan *imap.Message)
			trySelect(m, clientP, boxName, false)
			go func() {
				if err := clientP.c.UidFetch(seq, []imap.FetchItem{imap.FetchUid, imap.FetchInternalDate}, ch); err != nil {
					logger.Warn(err.Error())
				}
			}()
			sqlitedb.DB.DropTmpUid()
			sqlitedb.DB.InitTmpUid()
			for msg := range ch {
				t := model.TmpUid{
					Date: msg.InternalDate.Unix(),
					Uid:  msg.Uid,
				}
				if err := sqlitedb.DB.AddTmpUid(t); err != nil {
					print(err.Error())
				}
			}
			result := sqlitedb.DB.ListTmpUid()
			uids = utils.MapReduce[model.TmpUid, uint32](result, func(t model.TmpUid) uint32 {
				return t.Uid
			})
			sqlitedb.DB.DropTmpUid()
		}
		break
	}
	return
}
func tryFetchRecent(m *IMAP, clientP *IMAPClient, boxName string) {
	maxDate := int64(0)
	search := &imap.SearchCriteria{}
	dt := time.Now().Add(-30 * 24 * time.Hour)
	search.Since = dt
	ids, status := tryUidSearch(m, clientP, boxName, search)
	seq := &imap.SeqSet{}
	seq.AddNum(ids...)
	ch := make(chan *imap.Message)
	go clientP.c.UidFetch(seq, []imap.FetchItem{imap.FetchBody, imap.FetchEnvelope, imap.FetchUid, imap.FetchInternalDate, imap.FetchRFC822}, ch)
	for _mail := range ch {
		mail := fetchMail(_mail)
		if mail.InternalDate > maxDate {
			maxDate = mail.InternalDate
		}
		mail.Email = m.email
		mail.Mailbox = boxName
		sqlitedb.DB.AddMail(mail)
		logger.Info(fmt.Sprintf("new mail %d: %s", mail.Uid, mail.Subject))
	}
	sqlitedb.DB.UpAccountNext(m.param.Id, status.UidNext, boxName != m.spamName)
	if boxName == m.spamName {
		m.param.SpamUidNext = status.UidNext
	} else {
		m.param.InboxUidNext = status.UidNext
	}
	return
}
func FetchNewest(m *IMAP, clientP *IMAPClient, boxName string) error {
	uidNext := m.param.InboxUidNext
	if boxName == m.spamName {
		uidNext = m.param.SpamUidNext
	}
	status := trySelect(m, clientP, boxName, true)
	if status.UidNext == uidNext {
		return nil
	}
	search := &imap.SearchCriteria{}
	search.Since = time.Now().Add(-1 * 24 * time.Hour)
	uids, _ := tryUidSearch(m, clientP, boxName, search)
	if len(uids) > 0 {
		ch := make(chan *imap.Message)
		seq := &imap.SeqSet{}
		seq.AddNum(uids...)
		go clientP.c.UidFetch(seq, []imap.FetchItem{imap.FetchBody, imap.FetchEnvelope, imap.FetchUid, imap.FetchInternalDate, imap.FetchRFC822}, ch)
		for _mail := range ch {
			mail := fetchMail(_mail)
			mail.Email = m.email
			mail.Mailbox = "INBOX"
			if boxName == m.spamName {
				mail.Mailbox = "SPAM"
			}
			if sqlitedb.DB.AddMail(mail) {
				logger.Info(fmt.Sprintf("new mail %d: %s", mail.Uid, mail.Subject))
				if len(Pool.webhooks) > 0 {
					callWebhooks(m, mail, Pool.webhooks)
				}
			}
		}

	}
	sqlitedb.DB.UpAccountNext(m.param.Id, status.UidNext, boxName != m.spamName)
	if boxName == m.spamName {
		m.param.SpamUidNext = status.UidNext
	} else {
		m.param.InboxUidNext = status.UidNext
	}
	return nil
}
