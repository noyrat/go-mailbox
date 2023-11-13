package sqlitedb

import (
	"fmt"
	"mailbox/pkg/model"
	"mailbox/pkg/utils"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type SqliteDB struct {
	db *gorm.DB
}

var DB = CreateDB()

func CreateDB() *SqliteDB {
	db, _ := gorm.Open(sqlite.Open("data.sqlite"), &gorm.Config{
		//Logger: logger.Default.LogMode(logger.Info),
	})
	db.AutoMigrate(&model.Config{})
	db.AutoMigrate(&model.Mail{})
	db.AutoMigrate(&model.MailAccount{})
	db.AutoMigrate(&model.TmpUid{})
	db.AutoMigrate(&model.Webhook{})
	return &SqliteDB{db: db}
}
func (d *SqliteDB) GetMailAccount(id int) *model.MailAccount {
	var a model.MailAccount
	d.db.First(&a, id)
	return &a
}
func (d *SqliteDB) UpMailAccount(m model.MailAccount) {
	d.db.Save(&m)
}
func (d *SqliteDB) MailAccountList(status int) (ret []model.MailAccount) {
	if status != 0 {
		d.db.Where(&model.MailAccount{Status: status}).Find(&ret)
	} else {
		d.db.Find(&ret)
	}
	return
}
func (d *SqliteDB) AddMail(mail *model.Mail) bool {
	count := int64(0)
	d.db.Where(&model.Mail{Email: mail.Email, Mailbox: mail.Mailbox, Uid: mail.Uid}).Count(&count)
	if count == 0 {
		for {
			c := int64(0)
			mail.Token = utils.RandStr(16)
			if d.db.Where(&model.Mail{Token: mail.Token}).Count(&c); c == 0 {
				break
			}
		}
		err := d.db.Create(mail).Error
		if err != nil {
			print(err.Error())
			return false
		}
		return true
	}
	return false
}

func (d *SqliteDB) CleanMail() {
	var mails []model.Mail
	dt := time.Now().Unix()
	d.db.Where(&model.Mail{InternalDate: dt - (30 * 24 * 60 * 60)}).Find(&mails)
	if len(mails) > 0 {
		for _, m := range mails {
			d.db.Delete(m)
		}
	}
	return
}
func (d *SqliteDB) Mail(r model.MailRequest) *model.Mail {
	var ret model.Mail
	d.db.Where(&model.Mail{Id: r.Uid}).First(&ret)
	return &ret
}
func (d *SqliteDB) QuickMail(token string) *model.Mail {
	var ret model.Mail
	d.db.Where(&model.Mail{Token: token}).First(&ret)
	return &ret
}

func (d *SqliteDB) MailPage(r model.MailPageRequest) (ret model.Page[model.Mail]) {
	tx := d.db.Model(&model.Mail{})
	if r.DateStart != "" {
		if ds, e := time.Parse("2006-01-02 15:04:05", r.DateStart); e != nil {
			tx = tx.Where("internal_date >= ?", ds.Unix())
		}
	}
	if r.DateEnd != "" {
		if de, e := time.Parse("2006-01-02 15:04:05", r.DateEnd); e != nil {
			de.Add(3600*24 - 1*time.Second)
			tx = tx.Where("internal_date < ?", de.Unix())
		}
	}
	if r.BoxName != "ALL" {
		tx = tx.Where("mailbox = ?", r.BoxName)
	}
	if len(r.QueryFields) > 0 && r.Query != "" {
		sub := d.db.Where(&model.Mail{})
		for _, qf := range r.QueryFields {
			if qf == "subject" || qf == "body" {
				sub = sub.Where("subject", r.Query)
			}
			if qf == "from" || qf == "body" {
				sub = sub.Where(d.db.Where("from_name like ?", fmt.Sprintf("%%%s%%", r.Query)).Or("from_address like ?", fmt.Sprintf("%%%s%%", r.Query)))
			}
			if qf == "to" || qf == "body" {
				sub = sub.Where(d.db.Where("to_name like ?", fmt.Sprintf("%%%s%%", r.Query)).Or("to_address like ?", fmt.Sprintf("%%%s%%", r.Query)))
			}
			if qf == "cc" || qf == "body" {
				sub = sub.Where(d.db.Where("cc_name like ?", fmt.Sprintf("%%%s%%", r.Query)).Or("cc_address like ?", fmt.Sprintf("%%%s%%", r.Query)))
			}
			if qf == "bcc" || qf == "body" {
				sub = sub.Where(d.db.Where("bcc_name like ?", fmt.Sprintf("%%%s%%", r.Query)).Or("bcc_address like ?", fmt.Sprintf("%%%s%%", r.Query)))
			}
			if qf == "text" || qf == "body" {
				sub = sub.Where("text like ?", fmt.Sprintf("%%%s%%", r.Query))
			}
		}
		tx = tx.Where(sub)
	}
	total := int64(0)
	tx.Count(&total)
	list := []model.Mail{}
	if err := tx.Offset((r.PageNum - 1) * r.PageSize).Limit(r.PageSize).Order("internal_date desc").Find(&list).Error; err != nil {
		print(err.Error())
	}
	ret.Total = int(total)
	ret.List = list
	return
}

func (d *SqliteDB) DelMailAccount(id int) bool {
	return d.db.Delete(&model.MailAccount{Id: id}).Error == nil
}
func (d *SqliteDB) UpAccountStatus(id int, status int, msg string) {
	d.db.Where(&model.MailAccount{Id: id}).Updates(&model.MailAccount{Status: status, Info: msg})
}
func (d *SqliteDB) UpAccountNext(id int, uidNext uint32, isInbox bool) {
	if isInbox {
		d.db.Where(&model.MailAccount{Id: id}).Updates(&model.MailAccount{InboxUidNext: uidNext})
	} else {
		d.db.Where(&model.MailAccount{Id: id}).Updates(&model.MailAccount{SpamUidNext: uidNext})
	}
}

func (d *SqliteDB) WebhookList() (ret []model.Webhook) {
	d.db.Find(&ret)
	return
}
func (d *SqliteDB) GetWebhook(id int) *model.Webhook {
	var w model.Webhook
	d.db.First(w, id)
	return &w
}
func (d *SqliteDB) DelWebhook(id int) bool {
	return d.db.Delete(&model.Webhook{Id: id}).Error == nil
}
func (d *SqliteDB) UpWebhook(w model.Webhook) {
	d.db.Save(&w)
}
func (d *SqliteDB) GetConfig() *model.Config {
	var c model.Config
	d.db.Where("id", 1).First(&c)
	if c.Id == 0 {
		return nil
	}
	return &c
}
func (d *SqliteDB) UpConfig(c model.Config) {
	c.Id = 1
	d.db.Save(&c)
}
func (d *SqliteDB) DropTmpUid() {
	d.db.Migrator().DropTable(&model.TmpUid{})
}
func (d *SqliteDB) InitTmpUid() {
	if err := d.db.AutoMigrate(&model.TmpUid{}); err != nil {
		print(err.Error())
	}
}
func (d *SqliteDB) AddTmpUid(t model.TmpUid) error {
	return d.db.Save(&t).Error
}
func (d *SqliteDB) ListTmpUid() (ret []model.TmpUid) {
	d.db.Order("date asc").Find(&ret)
	return ret
}
