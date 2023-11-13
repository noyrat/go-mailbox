package imaplib

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"mailbox/pkg/model"
	"mailbox/pkg/sqlitedb"
	"mailbox/pkg/utils"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap"
)

func ListMailBox(email string) (ret []*model.MailBox) {
	ret = make([]*model.MailBox, 0)
	if m, ok := Pool.pool[email]; ok {
		m.fetch.lock.Lock()
		defer m.fetch.lock.Unlock()
		boxCh := make(chan *imap.MailboxInfo)
		go func() {
			tryInit(m, m.fetch)
			if err := m.fetch.c.List("", "*", boxCh); err != nil {
				logger.Warn(fmt.Sprintf("%s: list box err: %s", m.email, err.Error()))
			}
		}()
		for _, m := range scanBox(boxCh) {
			ret = append(ret, convertMailbox(m))
		}
	}
	return
}

func Mail(ctx context.Context, email string, mailbox string, r model.MailRequest) (ret *model.Mail) {
	if m, ok := Pool.pool[email]; ok {
		seq := &imap.SeqSet{}
		seq.AddNum(uint32(r.Uid))
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			m.fetch.lock.Lock()
			defer m.fetch.lock.Unlock()
			ch := make(chan *imap.Message)
			trySelect(m, m.fetch, r.BoxName, false)
			go func() {
				if err := m.fetch.c.UidFetch(seq, []imap.FetchItem{imap.FetchBody, imap.FetchEnvelope, imap.FetchUid, imap.FetchInternalDate, imap.FetchRFC822}, ch); err != nil {
					logger.Warn(err.Error())
				}
			}()
			for _mail := range ch {
				ret = fetchMail(_mail)
				ret.Email = m.email
				ret.Mailbox = mailbox
				break
			}
			wg.Done()
		}()
		wg.Wait()
		return
	}
	return nil
}
func MailPage(email string, r model.MailPageRequest) (ret model.Page[model.Mail]) {
	if m, ok := Pool.pool[email]; ok {
		hash := hex.EncodeToString(md5.New().Sum([]byte(fmt.Sprintf("%d|%s|%s|%s|%s|%s",
			r.AccountId, r.BoxName, r.DateEnd, r.DateStart, r.Query, strings.Join(r.QueryFields, ",")))))
		var ids []uint32
		var searchContinue bool
		m.fetch.lock.Lock()
		defer m.fetch.lock.Unlock()
		if cacheItem := Pool.seqCache.Get(fmt.Sprintf("%s@%s", email, r.BoxName)); cacheItem != nil && cacheItem.Value() != nil {
			if _ids, ok := cacheItem.Value()[hash]; ok {
				ids = _ids
				searchContinue = true
			}
		}
		if !searchContinue || len(ids) == 0 {
			search := imap.NewSearchCriteria()
			if r.Query != "" {
				or := make([]*imap.SearchCriteria, 0)
				for _, qf := range r.QueryFields {
					if utils.IndexOf([]string{"subject", "from", "to", "cc", "bcc", "text", "body"}, qf) == -1 {
						continue
					}
					sr := imap.NewSearchCriteria()
					if qf == "subject" {
						sr.Header.Add("SUBJECT", r.Query)
						or = append(or, sr)
					}
					if qf == "from" {
						sr.Header.Add("FROM", r.Query)
						or = append(or, sr)
					}
					if qf == "to" {
						sr.Header.Add("TO", r.Query)
						or = append(or, sr)
					}
					if qf == "cc" {
						sr.Header.Add("CC", r.Query)
						or = append(or, sr)
					}
					if qf == "bcc" {
						sr.Header.Add("BCC", r.Query)
						or = append(or, sr)
					}
					if qf == "text" {
						sr.Text = []string{r.Query}
						or = append(or, sr)
					}
					if qf == "body" {
						sr.Body = []string{r.Query}
						or = append(or, sr)
					}
				}
				if len(or) == 1 {
					search = or[0]
				} else if len(or) > 1 {
					search = or[0]
					for i := 1; i < len(or); i++ {
						base := new(imap.SearchCriteria)
						*base = *search
						union := imap.NewSearchCriteria()
						union.Or = [][2]*imap.SearchCriteria{{base, or[i]}}
						*search = *union
					}
				}
			}
			if r.DateStart != "" {
				if ds, e := time.Parse("2006-01-02 15:04:05", r.DateStart); e != nil {
					search.Since = ds
				}
			}
			if r.DateEnd != "" {
				if de, e := time.Parse("2006-01-02 15:04:05", r.DateEnd); e != nil {
					de = de.Add(3600*24 - 1*time.Second)
					search.Before = de
				}
			} else {
				search.Before = time.Now().Add(24 * time.Hour)
			}
			ids, _ = tryUidSearch(m, m.fetch, r.BoxName, search)
		}
		if !searchContinue && len(ids) > 0 {
			ids = utils.Reverse(ids)
			m := make(map[string][]uint32, 0)
			m[hash] = ids
			Pool.seqCache.Set(fmt.Sprintf("%s@%s", email, r.BoxName), m, 300*time.Second)
		} else if !searchContinue && len(ids) == 0 {
			ret.List = []model.Mail{}
			ret.Total = 0
			return
		}
		//sort
		//skip && limit
		min := (r.PageNum - 1) * r.PageSize
		max := r.PageNum * r.PageSize
		if max > len(ids)-1 {
			max = len(ids)
		}
		seq := &imap.SeqSet{}
		seq.AddNum(ids[min:max]...)
		ch := make(chan *imap.Message)
		go func() {
			//select?
			trySelect(m, m.fetch, r.BoxName, false)
			if err := m.fetch.c.UidFetch(seq, []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid, imap.FetchInternalDate}, ch); err != nil {
				logger.Warn(err.Error())
			}
		}()
		messageList := []model.Mail{}
		for m := range ch {
			mail := &model.Mail{
				Uid:          m.Uid,
				InternalDate: m.InternalDate.Unix(),
			}
			convertEnvelope(mail, m.Envelope)
			messageList = append(messageList, *mail)
		}
		sort.Slice(messageList[:], func(i, j int) bool {
			return messageList[i].InternalDate > messageList[j].InternalDate
		})
		ret.Total = len(ids)
		ret.List = messageList
	}
	return
}
func Clean() {
	ticker := time.NewTicker(24 * time.Hour)
	for {
		select {
		case <-ticker.C:
			sqlitedb.DB.CleanMail()
		}
	}
}
