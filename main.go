package main

import (
	"crypto/md5"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"mailbox/pkg/imaplib"
	"mailbox/pkg/model"
	"mailbox/pkg/oauthlib"
	"mailbox/pkg/sqlitedb"
	"mailbox/pkg/utils"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/jellydator/ttlcache/v3"
)

var cache = ttlcache.New[string, int]()

func Auth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		auth, _ := ctx.Cookie("token")
		if strings.HasPrefix(ctx.Request.URL.Path, "/api") {
			if strings.HasPrefix(ctx.Request.URL.Path, "/api/auth") {
				return
			}
			if strings.HasPrefix(ctx.Request.URL.Path, "/api/cache/") {
				return
			}
			if auth == "" {
				ctx.AbortWithStatus(401)
			} else if cache.Get(auth) != nil {
				return
			} else {
				config := sqlitedb.DB.GetConfig()
				if config != nil {
					hash := hex.EncodeToString(md5.New().Sum([]byte(config.User + "|go|@#$%|mailbox|" + config.Passwd)))
					if hash == auth {
						cache.Set(hash, 1, 99*365*24*time.Hour)
						return
					}
				}
				ctx.AbortWithStatus(401)
			}
		}
	}
}
func SPAMiddleware(urlPrefix, spaDirectory string) gin.HandlerFunc {
	directory := static.LocalFile(spaDirectory, true)
	fileserver := http.FileServer(directory)
	if urlPrefix != "" {
		fileserver = http.StripPrefix(urlPrefix, fileserver)
	}
	return func(c *gin.Context) {
		if count := strings.Count(c.Request.URL.Path, "/"); count <= 1 {
			c.Request.URL.Path = "/"
			fileserver.ServeHTTP(c.Writer, c.Request)
			c.Abort()
		}
	}
}

/*
	func timeoutResponse(c *gin.Context) {
		c.String(http.StatusRequestTimeout, "timeout")
	}

	func timeoutMiddleware() gin.HandlerFunc {
		return timeout.New(
			timeout.WithTimeout(10*time.Second),
			timeout.WithHandler(func(c *gin.Context) {
				c.Next()
			}),
			timeout.WithResponse(timeoutResponse),
		)
	}
*/
func main() {
	imaplib.Init()
	r := gin.Default()
	/*
		r.Use(gin.BasicAuth(gin.Accounts{
			"admin": "random056",
		}))*/
	for _, path := range []string{"/", "/mail", "/login", "/logout", "/register",
		"/setting", "/setting/:item",
		"/cache/:token"} {
		r.GET(path, func(ctx *gin.Context) {
			ctx.File("web/dist/index.html")
		})
	}
	r.StaticFS("/assets", http.Dir("web/dist/assets"))
	//spa
	//r.Use(SPAMiddleware("/", "web"))
	r.Use(Auth())
	api := r.Group("/api")
	//api.Use(timeoutMiddleware())
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, model.Res{
			Data: "ok",
		})
	})
	api.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, imaplib.Status())
	})
	r.GET("/oauth2/:site/callback", func(c *gin.Context) {
		site := c.Param("site")
		var r model.OAuthRequest
		var existAccount *model.MailAccount
		c.ShouldBindQuery(&r)
		if site == "" {
			c.Redirect(301, "/setting/mailAccount?callback=error&msg=site%20missing")
			return
		}
		if r.Code == "" {
			c.Redirect(301, "/setting/mailAccount?callback=error&msg=code%20missing")
			return
		}
		if r.State != "" {
			accountId, _ := strconv.Atoi(r.State)
			existAccount = sqlitedb.DB.GetMailAccount(accountId)
		}
		var h oauthlib.IHandler
		if site == "google" {
			h = &oauthlib.GoogleHandler{}
		} else if site == "outlook" {
			h = &oauthlib.OutlookHandler{}
		}
		account, err := h.GetAccount(r.Code)
		if err != nil {
			c.Redirect(301, "/setting/mailAccount?callback=error&msg="+url.QueryEscape(err.Error()))
			return
		}
		if _, err := imaplib.ImapLogin(account); err != nil {
			c.Redirect(301, "/setting/mailAccount?callback=error&msg="+url.QueryEscape(err.Error()))
			return
		}
		if existAccount != nil {
			if existAccount.Email != account.Email {
				c.Redirect(301, "/setting/mailAccount?callback=error&msg="+url.QueryEscape("account not match"))
				return
			}
			imaplib.SwitchMailAccount(existAccount, 2)
			account.InboxUidNext = existAccount.InboxUidNext
			account.SpamUidNext = existAccount.SpamUidNext
			account.Id = existAccount.Id
			account.Status = 1
		}
		sqlitedb.DB.UpMailAccount(*account)
		go imaplib.StartMailAccount(*account)
		c.Redirect(301, "/setting/mailAccount?callback=success")
	})
	//auth
	api.POST("/auth/status", func(c *gin.Context) {
		auth, _ := c.Cookie("token")
		config := sqlitedb.DB.GetConfig()
		//未登录
		status := -1
		if config == nil {
			//未注册
			status = 0
		} else if auth != "" {
			if v := cache.Get(auth); v != nil {
				//已登录
				status = 1
			} else {
				hash := hex.EncodeToString(md5.New().Sum([]byte(config.User + "|go|@#$%|mailbox|" + config.Passwd)))
				if hash == auth {
					cache.Set(hash, 1, 99*365*24*time.Hour)
					status = 1
				}
			}
		}
		if status != 1 {
			config = nil
		} else {
			config.Passwd = ""
			config.Salt = ""
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: map[string]interface{}{
				"logined": status,
				"config":  config,
			},
		})
	})
	api.POST("/auth/logout", func(c *gin.Context) {
		for _, cookie := range c.Request.Cookies() {
			c.SetCookie(cookie.Name, "", -1, cookie.Path, cookie.Domain, true, true)
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
		})
	})
	api.POST("/auth/login", func(c *gin.Context) {
		var r model.LoginRequest
		_ = c.ShouldBindJSON(&r)
		config := sqlitedb.DB.GetConfig()
		if config != nil && r.Username != "" && r.Password != "" {
			if r.Username != config.User || hex.EncodeToString(sha512.New().Sum([]byte(r.Password+config.Salt))) != config.Passwd {
				c.JSON(http.StatusOK, model.Res{
					Code: -1,
					Msg:  "账号密码错误",
				})
				return
			}
			hash := hex.EncodeToString(md5.New().Sum([]byte(config.User + "|go|@#$%|mailbox|" + config.Passwd)))
			cache.Set(hash, 1, 99*365*24*time.Hour)
			c.SetCookie("token", hash, 99*365*24*60*60, "/", c.Request.Host, true, true)
			c.JSON(http.StatusOK, model.Res{
				Code: 0,
			})
		} else {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Msg:  "用户不存在",
			})
		}
	})
	api.POST("/auth/register", func(c *gin.Context) {
		var r model.LoginRequest
		_ = c.ShouldBindJSON(&r)
		config := sqlitedb.DB.GetConfig()
		if config != nil {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Msg:  "用户已注册",
			})
			return
		}
		if r.Username == "" && r.Password == "" {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Msg:  "无效输入",
			})
			return
		}
		config = &model.Config{
			User:   r.Username,
			Passwd: r.Password,
			Salt:   utils.RandStr(12),
		}
		config.Passwd = hex.EncodeToString(sha512.New().Sum([]byte(r.Password + config.Salt)))
		sqlitedb.DB.UpConfig(*config)
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
		})
	})
	api.POST("/config/auth", func(c *gin.Context) {
		var r model.ConfigAuthRequest
		config := sqlitedb.DB.GetConfig()
		c.ShouldBindJSON(&r)
		//auth
		if r.Username == "" || r.Password == "" || r.NewPassword == "" {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Msg:  "参数错误",
			})
			return
		}
		if hex.EncodeToString(sha512.New().Sum([]byte(r.Password+config.Salt))) != config.Passwd {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Msg:  "密码错误",
			})
			return
		}
		config.User = r.Username
		config.Passwd = r.NewPassword
		config.Salt = utils.RandStr(12)
		config.Passwd = hex.EncodeToString(sha512.New().Sum([]byte(r.NewPassword + config.Salt)))
		sqlitedb.DB.UpConfig(*config)
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
		})
	})
	api.POST("/config/oauth", func(c *gin.Context) {
		var r model.ConfigOAuthRequest
		config := sqlitedb.DB.GetConfig()
		c.ShouldBindJSON(&r)
		config.GoogleClientID = r.GoogleClientID
		config.GoogleClientSecret = r.GoogleClientSecret
		config.GoogleRedirectUri = r.GoogleRedirectUri

		config.OutlookClientID = r.OutlookClientID
		config.OutlookClientSecret = r.OutlookClientSecret
		config.OutlookRedirectUri = r.OutlookRedirectUri
		sqlitedb.DB.UpConfig(*config)
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
		})
	})
	api.GET("/mailaccount", func(c *gin.Context) {
		s := c.Param("s")
		status, _ := strconv.Atoi(s)
		mailboxs := sqlitedb.DB.MailAccountList(status)
		for _, b := range mailboxs {
			b.Passwd = ""
		}
		c.JSON(http.StatusOK, model.Res{
			Data: mailboxs,
		})
	})
	api.DELETE("/mailaccount/:id", func(c *gin.Context) {
		_id := c.Param("id")
		id, _ := strconv.Atoi(_id)
		mailbox := sqlitedb.DB.GetMailAccount(id)
		if mailbox != nil {
			imaplib.SwitchMailAccount(mailbox, 2)
			ret := sqlitedb.DB.DelMailAccount(id)
			if ret {
				c.JSON(http.StatusOK, model.Res{
					Code: 0,
					Data: true,
				})
				return
			}
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: false,
		})
	})
	api.POST("/mailaccount", func(c *gin.Context) {
		var m model.MailAccount
		if err := c.ShouldBindJSON(&m); err != nil {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Data: nil,
				Msg:  "param error",
			})
			return
		}
		if m.Id > 0 {
			mailbox := sqlitedb.DB.GetMailAccount(m.Id)
			if mailbox != nil {
				imaplib.SwitchMailAccount(mailbox, 2)
			}
		}
		sqlitedb.DB.UpMailAccount(m)
		imaplib.StartMailAccount(m)
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
		})
	})

	api.POST("/mailaccount/check", func(c *gin.Context) {
		var m model.MailAccount
		if err := c.ShouldBindJSON(&m); err != nil {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Data: nil,
				Msg:  "param error",
			})
			return
		}
		_, err := imaplib.ImapLogin(&m)
		if err != nil {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Msg:  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: nil,
		})
	})
	api.POST("/mailaccount/switch", func(c *gin.Context) {
		var m model.MailAccountSwitchRequest
		if err := c.ShouldBindJSON(&m); err != nil || m.AccountId == 0 || m.Status > 2 || m.Status < 1 {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Data: nil,
				Msg:  "param error",
			})
			return
		}
		account := sqlitedb.DB.GetMailAccount(m.AccountId)
		if account == nil {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Data: nil,
				Msg:  "account not found",
			})
			return
		}
		err := imaplib.PreSwitchMailAccount(account, m.Status)
		if err == nil {
			err = imaplib.SwitchMailAccount(account, m.Status)
		}
		if err != nil {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Data: nil,
				Msg:  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: nil,
		})
	})
	//region mailbox
	api.GET("/mailbox/:id", func(c *gin.Context) {
		_id := c.Param("id")
		id, _ := strconv.Atoi(_id)
		mailAccount := sqlitedb.DB.GetMailAccount(id)
		data := make([]*model.MailBox, 0)
		if mailAccount != nil {
			data = imaplib.ListMailBox(mailAccount.Email)
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: data,
		})
	})
	//region mail
	api.POST("/maillist", func(c *gin.Context) {
		var r model.MailPageRequest
		_ = c.ShouldBindJSON(&r)
		if r.PageNum <= 0 {
			r.PageNum = 1
		}
		if r.PageSize <= 0 {
			r.PageSize = 50
		}
		var page model.Page[model.Mail]
		if r.AccountId == 0 {
			page = sqlitedb.DB.MailPage(r)
		} else {
			mailAccount := sqlitedb.DB.GetMailAccount(r.AccountId)
			if mailAccount != nil {
				page = imaplib.MailPage(mailAccount.Email, r)
			}
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: page,
		})
	})
	api.POST("/mail", func(c *gin.Context) {
		var r model.MailRequest
		_ = c.ShouldBindJSON(&r)
		var mail *model.Mail
		if r.AccountId == 0 {
			mail = sqlitedb.DB.Mail(r)
		} else if mailAccount := sqlitedb.DB.GetMailAccount(r.AccountId); mailAccount != nil {
			mail = imaplib.Mail(c, mailAccount.Email, r.BoxName, r)
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: mail,
		})
	})
	api.GET("/cache/:token", func(c *gin.Context) {
		token := c.Param("token")
		mail := sqlitedb.DB.QuickMail(token)
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: mail,
		})
	})

	// region webhook
	api.GET("/webhook", func(c *gin.Context) {
		webhooks := sqlitedb.DB.WebhookList()
		c.JSON(http.StatusOK, model.Res{
			Data: webhooks,
		})
	})
	api.DELETE("/webhook/:id", func(c *gin.Context) {
		_id := c.Param("id")
		id, _ := strconv.Atoi(_id)
		webhook := sqlitedb.DB.GetWebhook(id)
		if webhook != nil {
			imaplib.DelWebhook(*webhook)
			ret := sqlitedb.DB.DelWebhook(id)
			if ret {
				c.JSON(http.StatusOK, model.Res{
					Code: 0,
					Data: true,
				})
				return
			}
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: false,
		})
	})
	api.POST("/webhook", func(c *gin.Context) {
		var h model.Webhook
		if err := c.ShouldBindJSON(&h); err != nil {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Data: nil,
				Msg:  "param error",
			})
			return
		}
		if h.Id > 0 {
			webhook := sqlitedb.DB.GetWebhook(h.Id)
			if webhook != nil {
				imaplib.DelWebhook(*webhook)
			}
		}
		sqlitedb.DB.UpWebhook(h)
		imaplib.AddWebhook(h)
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
		})
	})

	api.POST("/webhook/check", func(c *gin.Context) {
		var h model.Webhook
		var header map[string]string
		if err := c.ShouldBindJSON(&h); err != nil {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Data: nil,
				Msg:  "param error",
			})
			return
		}
		var errMsg = ""
		if h.Method != "GET" && h.Method != "POST" {
			errMsg = "method error"
		} else if len(h.Url) == 0 {
			errMsg = "url error"
		} else if len(h.Header) > 0 {
			if err := json.Unmarshal([]byte(h.Header), &header); err != nil {
				errMsg = "header error"
			}
		} else if len(h.Filter) > 0 {
			for _, f := range h.Filter {
				if utils.IndexOf([]string{"Email", "From", "To", "Mailbox", "Subject", "Text", "Html"}, f.Variable) == -1 {
					errMsg = "filter variable error"
					break
				} else if utils.IndexOf([]string{"=", "Include", "Exclude", "RegExp"}, f.Match) == -1 {
					errMsg = "filter match type error"
					break
				} else if len(f.Param) == 0 {
					errMsg = "filter param error"
					break
				}
			}
		}
		if errMsg != "" {
			c.JSON(http.StatusOK, model.Res{
				Code: -1,
				Data: nil,
				Msg:  errMsg,
			})
			return
		}
		err := imaplib.CheckWebhook(h, header)
		if err != nil {
			c.JSON(http.StatusOK, model.Res{
				Code: 0,
				Data: "webhook check failed",
			})
			return
		}
		c.JSON(http.StatusOK, model.Res{
			Code: 0,
			Data: nil,
		})
	})
	// endregion
	r.Run("localhost:3000")
}
