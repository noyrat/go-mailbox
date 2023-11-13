var search = window.location.search
if (search.startsWith("?")){
    search = search.substring(1)
}
window.store = {
    accountId:null,
    boxName:null,
    search:null,
    mailId:null,
    pageSize:20,
    pageTotal:0,
    searchParams:deparam(search),
    loadCount:0,
    mail:{
        mailBoxList:[],
        mailList:[],
    },
    setting:{
        config:{},
        webhooks:[],
        mailAccounts:[],
    },
    backUrl:null,
    smup:false,
}
const preset = {
    setting:{
        menu:[
            {name:"登录设置",icon:"account_circle",href:"/setting/login"},
            {name:"OAuth设置",icon:"people",href:"/setting/oauth"},
            {name:"账户管理",icon:"manage_accounts",href:"/setting/mailAccount"},
            {name:"WebHook",icon:"link",href:"/setting/webhook"},
        ]
    }
}
function dateFormat(date,fmt){
    var o={
        "M+":date.getMonth()+1,//月份
        "d+":date.getDate(),//日
        "h+":date.getHours(),//小时
        "m+":date.getMinutes(),//分
        "s+":date.getSeconds(),//秒
        "q+":Math.floor((date.getMonth()+3)/3),//季度
        "S":date.getMilliseconds()//毫秒
    };
    if(/(y+)/.test(fmt))
        fmt=fmt.replace(RegExp.$1,(date.getFullYear()+"").substr(4-RegExp.$1.length));
    for(var k in o) {
        if(new RegExp("("+k+")").test(fmt))
        fmt=fmt.replace(RegExp.$1,(RegExp.$1.length==1)?(o[k]):(("00"+o[k]).substr((""+o[k]).length)));
    }
    return fmt;
}
//$.router.set("/...")
function loading(){
    window.store.loadCount++;
    if ($("#loading").attr('open') != 'true'){
        $("#loading").attr('open',true)
    }
}
function stopLoading(){
    window.store.loadCount--;
    if (window.store.loadCount == 0 && $("#loading").attr('open') == 'open'){
        $("#loading").attr('open',false)
    }
}
var api = {
    async call(f,
        successCallback,
        errorCallback=async (r)=>{
            mdui.snackbar({
                message: r.msg,
                placement:"top",
                autoCloseDelay: 1000
            });
        }
    ){
        let res
        try{
            res = await f
        }catch(e){
            res = {code:-1,msg:"request error"}
        }
        if (res.code == 0){
            await successCallback(res)
        }else{
            await errorCallback(res)
        }
    },
    async login(username,password){
        let r = await fetch("/api/auth/login",{
            method:"POST",
            headers:{
                "Content-Type":"application/json",
            },
            body:JSON.stringify({
              "u":username,
              "p":password,
            })
        })
        return await r.json()
    },
    async register(username,password){
        let r = await fetch("/api/auth/register",{
            method:"POST",
            headers:{
                "Content-Type":"application/json",
            },
            body:JSON.stringify({
              "u":username,
              "p":password,
            })
        })
        return await r.json()
    },
    async logout(){
        let r = await fetch("/api/auth/logout",{
            method:"POST",
            headers:{
                "Content-Type":"application/json",
            },
            body:JSON.stringify({
            })
        })
        return await r.json()
    },
    async status(){
        let r = await fetch("/api/auth/status",{
            method:"POST",
            headers:{
                "Content-Type":"application/json",
            },
            body:JSON.stringify({})
        })
        return await r.json()
    },
    mail:{
        async mailBoxList(accountId){
            if (accountId == 0){
                return {code:0,data:[{n:"ALL"},{n:"INBOX"},{n:"SPAM",a:["\\Junk"]}]}
            }
            let r = await fetch("/api/mailbox/"+accountId,{
                method:"GET"
            })
            let j = await r.json()
            if (j.code == 0){
                j.data.sort((a, b) => {
                    return (a.n < b.n)?-1:(a.n > b.n)?1:0;
                })
                let topList = {
                    "INBOX":null,
                    "SPAM":null,
                    "SENT":null,
                    "DRAFTS":null,
                    "TRASH":null,
                    "ARCHIVE":null,
                }
                let list = []
                for (var o of j.data){
                    if (o.n == "INBOX"){topList["INBOX"]=o;continue}
                    let taged = false
                    if (o.a.length > 0){
                        for (var ao of o.a){
                            if (ao == "\\Archive"){topList["ARCHIVE"] = o;taged=true;break}
                            if (ao == "\\Drafts"){topList["DRAFTS"] = o;taged=true;break}
                            if (ao == "\\Sent"){topList["SENT"] = o;taged=true;break}
                            if (ao == "\\Junk"){topList["SPAM"] = o;taged=true;break}
                            if (ao == "\\Trash"){topList["TRASH"] = o;taged=true;break}
                        }
                    }
                    if (!taged){list.push(o)}
                }
                for (var k of ["ARCHIVE","SPAM","DRAFTS","TRASH","SENT","INBOX"]){
                    if (topList[k] != null){
                        list.unshift(topList[k])
                    }
                }
                j.data = list
            }
            return j
        },
        async mailList(accountId,boxName,pageNum,pageSize,query,queryFields){
            let url = `/api/maillist`
            let r = await fetch(url,{
                method:"POST",
                headers:{
                    "Content-Type":"application/json",
                },
                body:JSON.stringify({
                    "i":accountId,
                    "n":boxName,
                    "pn":pageNum,
                    "ps":pageSize,
                    "q":query,
                    "qf":queryFields,
                })
            })
            return await r.json()
        },
        async mail(accountId,boxName,uid){
            let url = `/api/mail`
            let r = await fetch(url,{
                method:"POST",
                headers:{
                    "Content-Type":"application/json",
                },
                body:JSON.stringify({
                    "i":accountId,
                    "n":boxName,
                    "u":uid,
                })
            })
            return await r.json()
        },
    },
    setting:{
        async saveOAuth(data){
            let r = await fetch("/api/config/oauth",{
                method:"POST",
                headers:{
                    "Content-Type":"application/json",
                },
                body:JSON.stringify({
                    "gid":data.gid,
                    "gs":data.gs,
                    "gu":data.gu,
                    "oid":data.oid,
                    "os":data.os,
                    "ou":data.ou,
                })
            })
            return await r.json()
        },
        async mailAccountList(){
            let r = await fetch("/api/mailaccount",{
                method:"GET"
            })
            return await r.json()
        },
        async checkMailAccount(account){
            let r = await fetch("/api/mailaccount/check",{
                method:"POST",
                mode:"cors",
                headers:{
                    "Content-Type":"application/json",
                },
                body:JSON.stringify({
                    "e":account.email,
                    "pw":account.passwd,
                    "h":account.host,
                    "p":parseInt(account.port),
                    "t":account.type
                })
            })
            return await r.json()
        },
        async deleteMailAccount(id){
            let r = await fetch("/api/mailaccount/"+id,{
                method:"DELETE",
                mode:"cors",
                headers:{
                    "Content-Type":"application/json",
                },
                body:JSON.stringify({
                })
            })
            return await r.json()
        },
        async switchMailAccount(id,s){
            let r = await fetch("/api/mailaccount/switch",{
                method:"POST",
                mode:"cors",
                headers:{
                    "Content-Type":"application/json",
                },
                body:JSON.stringify({
                    "i":id,
                    "s":s
                })
            })
            return await r.json()
        },
        async webhookList(){
            let r = await fetch("/api/webhook",{
                method:"GET"
            })
            return await r.json()
        },
        async checkWebhook(webhook){
            let r = await fetch("/api/webhook/check",{
                method:"POST",
                mode:"cors",
                headers:{
                    "Content-Type":"application/json",
                },
                body:JSON.stringify({
                    "n":webhook.name,
                    "m":webhook.method,
                    "f":webhook.filters,
                    "u":webhook.url,
                    "h":webhook.header,
                    "b":webhook.body
                })
            })
            return await r.json()
        },
        async saveWebhook(webhook){
            let r = await fetch("/api/webhook",{
                method:"POST",
                mode:"cors",
                headers:{
                    "Content-Type":"application/json",
                },
                body:JSON.stringify({
                    "i":webhook.id,
                    "n":webhook.name,
                    "m":webhook.method,
                    "f":webhook.filters,
                    "u":webhook.url,
                    "h":webhook.header,
                    "b":webhook.body
                })
            })
            return await r.json()
        },
        async deleteWebhook(id){
            let r = await fetch("/api/webhook/"+id,{
                method:"DELETE",
                mode:"cors",
                headers:{
                    "Content-Type":"application/json",
                },
                body:JSON.stringify({
                })
            })
            return await r.json()
        }
    },
    async oauth(site){
        let r = await fetch("/api/oauth/"+site,{
            method:"POST",
            headers:{
                "Content-Type":"application/json"
            },
            body:JSON.stringify({
                "c":searchParams.code??"",
            })
        })
        return await r.json()
    }
}
async function initMailBox(){
    loading()
    $("#mailbox-list mdui-list").html('')
    await api.call(api.mail.mailBoxList(window.store.searchParams.accountId??0),
        async (r)=> {
            for (var item of r.data){
                let icon = 'folder';
                let name = item.n
                if (r.data.indexOf(item)==0){
                    name = "收件箱"
                    icon = 'inbox';
                }
                if (item.a && item.a.length > 0){
                    for (var ao of item.a){
                        if (ao == "\\Trash"){
                            name = "回收站"
                            icon = 'delete';break;
                        }else if (ao == "\\Junk"){
                            name = "垃圾邮件"
                            icon = 'report';break;
                        }else if (ao == "\\Sent"){
                            name = "发件箱"
                            icon = 'send';break;
                        }else if (ao == "\\Drafts"){
                            name = "草稿"
                            icon = 'insert_drive_file';break;
                        }else if (ao == "\\Archive"){
                            name = "存档"
                            icon = 'archive';break;
                        }
                    }
                }
                let dom = $(
                `<mdui-list-item class="mail-box-item"
                    rounded 
                    ${item.n==(window.store.searchParams.boxName??"INBOX")?"active":""}
                </mdui-list-item>`)
                dom.attr('data-n',item.n)
                dom.attr("icon",icon)
                dom.attr("headline",name)
                $("#mailbox-list mdui-list").append(dom)
            }
        }
    )
    stopLoading()
}

async function initMailList(){
    loading()
    $("#mail-list mdui-list").html('')
    $("#mail-page-input").attr('placeholder','')
    var accountId = parseInt(window.store.searchParams.accountId??0)
    await api.call(api.mail.mailList(accountId,
        window.store.boxName,
        parseInt(window.store.searchParams.pageNum??"1"),
        window.store.pageSize,
        $("#btn-mail-search").parent().val()??'',['subject','from']),
        async (r)=> {
            $("#mail-page-control mdui-segmented-button").removeAttr("disabled")
            for (var item of r.data.list){
                /*
                rounded 
                ${item.n==(window.store.searchParams.boxName??"INBOX")?"active":""}
                */
                let dom = $(
                `<mdui-list-item class="mail-list-item"
                data-i="${accountId==0?item.i:item.u}"
                headline-line=1
                description-line=${accountId==0?"2":"1"}
                >
                    <span slot="description"></span>
                </mdui-list-item>`)
                let headlineDiv = $("<div class='flex row w100 space-between'></div>")
                $("<div class='bold fontsize08'></div>").text(item.fn.length>0&&item.fn[0]?item.fn[0]:item.fa[0]).appendTo(headlineDiv)
                $("<div class='fontsize07'></div>").text(dateFormat(new Date(item.idt*1000),"yyyy-MM-dd hh:mm:ss")).appendTo(headlineDiv)
                headlineDiv.appendTo(dom)
                if (accountId == 0){
                    dom.find("[slot='description']").append($("<span></span>").text(item.e)).append($("<br/>"))
                }
                dom.find("[slot='description']").append($("<span></span>").text(item.s))
                dom.attr("description",item.s)
                $("#mail-list mdui-list").append(dom)
            }
            $(`.mail-list-item[data-i='${window.store.searchParams.mailId}'`).attr('active','')
            window.store.pageTotal = Math.ceil(r.data.total/window.store.pageSize) 
            $("#mail-page-input").attr('placeholder',`${window.store.searchParams.pageNum??1} / ${window.store.pageTotal}`)
            if ((window.store.searchParams.pageNum??1) == 1){
                $('#mail-page-first').attr('disabled','')
                $('#mail-page-prev').attr('disabled','')
            }
            if ((window.store.searchParams.pageNum??1) == window.store.pageTotal){
                $('#mail-page-next').attr('disabled','')
                $('#mail-page-last').attr('disabled','')
            }
        }
    )
    stopLoading()
}
async function initMail(){
    loading()
    $('#mail-content').hide()
    await api.call(api.mail.mail(parseInt(window.store.accountId??0),
        (window.store.boxName??"INBOX"),
        parseInt(window.store.mailId)),
        async (r)=> {
            var mail = r.data
            if (mail == null){return}
            var div = $('#mail-content')
            div.find("[tag='subject']").text(mail.s)
            div.find("[tag='from'] div").html('')
            div.find("[tag='to'] div").html('')
            for (var i=0;i< r.data.fa.length;i++){
                var c = $(`<mdui-chip></mdui-chip>`)
                c.text(mail.fn[i]==""?r.data.fa[i]:(mail.fn[i] + `<${r.data.fa[i]}>`))
                c.appendTo(div.find("[tag='from'] div"))
            }
            for (var i=0;i< r.data.ta.length;i++){
                var c = $(`<mdui-chip></mdui-chip>`)
                c.text(mail.tn[i]==""?r.data.ta[i]:(mail.tn[i] + `<${r.data.ta[i]}>`))
                c.appendTo(div.find("[tag='to'] div"))
            }
            //content
            let textItems = div.find("[tag='text']").html('').hide()
            let htmlItems = div.find("[tag='html']").html('').hide()
            let btnGroup =  div.find("[tag='btn-group']")
            btnGroup.find("[value='text']").attr("disabled",'')
            btnGroup.find("[value='html']").attr("disabled",'')
            
            if (mail.t != null){
                btnGroup.find("[value='text']").removeAttr("disabled")
                btnGroup.attr('value','text')
                const Rexp = RegExp('(?:(?:https?|ftp)://)(?:\x5cS+(?::\x5cS*)?@)?(?:(?!(?:10|127)(?:\x5c.\x5cd{1,3}){3})(?!(?:169\x5c.254|192\x5c.168)(?:\x5c.\x5cd{1,3}){2})(?!172\x5c.(?:1[6-9]|2\x5cd|3[0-1])(?:\x5c.\x5cd{1,3}){2})(?:[1-9]\x5cd?|1\x5cd\x5cd|2[01]\x5cd|22[0-3])(?:\x5c.(?:1?\x5cd{1,2}|2[0-4]\x5cd|25[0-5])){2}(?:\x5c.(?:[1-9]\x5cd?|1\x5cd\x5cd|2[0-4]\x5cd|25[0-4]))|(?:(?:[a-z\x5cu00a1-\x5cuffff0-9]-*)*[a-z\x5cu00a1-\x5cuffff0-9]+)(?:\x5c.(?:[a-z\x5cu00a1-\x5cuffff0-9]-*)*[a-z\x5cu00a1-\x5cuffff0-9]+)*(?:\x5c.(?:[a-z\x5cu00a1-\x5cuffff]{2,}))\x5c.?)(?::\x5cd{2,5})?(?:[/?#]\x5cS*)?','gi')
                let cursor = 0;
                let match,index
                while (match = Rexp.exec(mail.t)) {
                    index = match.index
                    var span = $("<span></span>")
                    span.text(mail.t.substring(cursor,index))
                    textItems.append(span)
                    var anchor = $("<a></a>")
                    anchor.attr("href",match[0])
                    anchor.text(match[0])
                    textItems.append(anchor)
                    cursor = Rexp.lastIndex
                }
                let t = mail.t.substring(cursor,mail.t.length)
                var lastText = $("<span></span>")
                lastText.text(t)
                textItems.append(lastText)
            }
            if (mail.h != null && mail.h.length > 0){
                btnGroup.find("[value='html']").removeAttr("disabled")
                btnGroup.val('html')
                htmlItems.show()
                var template = document.createElement('template');
                var html = mail.h.trim();
                template.innerHTML = html;
                for (var s of template.querySelectorAll("script")){
                    s.remove()
                }
                template.innerHTML = template.innerHTML.replace("<","<script>window.setInterval(function(){window.frameElement.style.height=(document.body.scrollHeight+24)+'px';},1000)</script><style>html{overflow-y:hidden!important}</style><")
                //onload='javascript:(function(o){console.log(123);o.style.height=(o.contentWindow.document.body.scrollHeight+1)+"px";}(this));'
                var iframe = $("<iframe></iframe>")
                iframe.attr('srcdoc',template.innerHTML)
                iframe.attr('style',"width:100%;height:100%")
                htmlItems.append(iframe)
            }
            if (btnGroup.val()=='' || btnGroup.val() == 'text'){
                btnGroup.val('text')
                textItems.show()
            }
            if (mdui.breakpoint().down("sm")){
                $("#mail-container").animate({ scrollLeft: $("body").width()+"px" });
                $("mdui-top-app-bar").animate({ scrollLeft: $("#app-bar").width()/2 });
            }
            div.show()
        }
    )
    stopLoading()
}
async function initSetting(){
    $("#setting-list mdui-list").html('')
    for (var item of preset.setting.menu){
        let dom = $(
        `<mdui-list-item 
            rounded 
            ${item.href==window.location.pathname?"active":""} 
            onclick="javascript:$.router.set('${item.href}')">
            ${item.name}
            <mdui-icon slot="icon" name="${item.icon}"></mdui-icon>
        </mdui-list-item>`)
        $("#setting-list mdui-list").append(dom)
    }
}
async function initSettingLogin(){
    loading()
    var div = $("div[path='/setting/login']")
    var config = window.store.setting.config
    div.find('[label="username"]').val(config.u??"")
    stopLoading()
}
async function initSettingOAuth(){
    loading()
    var div = $("div[path='/setting/oauth']")
    var config = window.store.setting.config
    div.find('[label="Google Client Id"]').val(config.gid??"")
    div.find('[label="Google Client Secret"]').val(config.gs??"")
    div.find('[label="Google Redirect Url"]').val(config.gu??"")
    div.find('[label="Outlook Client Id"]').val(config.oid??"")
    div.find('[label="Outlook Client Secret"]').val(config.os??"")
    div.find('[label="Outlook Redirect Url"]').val(config.ou??"")
    stopLoading()
}
async function initSettingMailAccount(){
    $('#btn-mail-account-edit').attr("disabled",'')
    $('#btn-mail-account-delete').attr("disabled",'')
    loading()
    await api.call(api.setting.mailAccountList(),
        async (r)=>{
            window.store.setting.mailAccounts = r.data
            let div = $(`#setting-content>div[path='/setting/mailAccount']`);
            div.find("mdui-list").html('')
            var index = 0
            for (var item of r.data){
                let dom = $(
                    `<mdui-list-item 
                        class="mail-account-item" data-id="${index++}"
                        headline-line=1
                        description-line=1
                        >
                        ${item.e}
                        <mdui-switch slot="end-icon" ${item.s==2?"":"checked"}></mdui-switch>
                    </mdui-list-item>`)
                dom.attr("description",item.t.toUpperCase() +" "+ item.h+":"+item.p)
                div.find("mdui-list").append(dom)
            }
            index = -1
            let dropdown = $(`#dropdown-mail-account`);
            dropdown.find("mdui-menu").html('')
            for (var item of [{i:0,e:"全局收件箱"},...r.data]){
                if (item.i != 0 && item.s == 2){
                    index++
                    continue
                }
                if (String(item.i) == (window.store.accountId??'0')){
                    dropdown.find('mdui-text-field').attr('value',item.e)
                }
                let dom = $(
                    `<mdui-menu-item dense data-id="${index++}">
                    </mdui-menu-item>`)
                dom.text(item.e)
                dropdown.find("mdui-menu").append(dom)
            }
            if (window.store.searchParams.callback != null || window.store.searchParams.msg != null){
                hint(window.store.searchParams.msg??window.store.searchParams.callback)
            }
        })
    stopLoading()
}
async function initSettingWebhook(){
    $('#btn-webhook-edit').attr("disabled",'')
    $('#btn-webhook-delete').attr("disabled",'')
    loading()
    await api.call(api.setting.webhookList(),
        async (r)=>{
            window.store.setting.webhook = r.data
            let div = $(`#setting-content>div[path='/setting/webhook']`);
            div.find("mdui-list").html('')
            var index = 0
            for (var item of r.data){
                //  ${item.href==window.location.pathname?"active":""} 
                let dom = $(
                    `<mdui-list-item 
                        class="webhook-item" data-id="${index++}"
                        headline-line=1
                        description-line=1
                        >
                        ${item.m.toUpperCase() + " " + item.n}
                    </mdui-list-item>`)
                dom.attr("description",item.u)
                div.find("mdui-list").append(dom)
            }
        })
    stopLoading()
}
async function init(){
    // init theme
    if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches){}
    // init width
    if (window.innerWidth > 600) {
        $('head').append($(`<style>.sleft{width:300px}</style>`))
    }
    loading()
    await api.call(api.status(),
        async (r)=>{
            if (r.data.logined == -1){  //未登录
                $.router.set("/login")
            }else if (r.data.logined == 1){ //已登录
                await initSettingMailAccount()
                if (r.data != null && r.data.config){
                    r.data.config.gu = window.origin + "/oauth2/google/callback"
                    r.data.config.ou = window.origin + "/oauth2/outlook/callback"
                    window.store.setting.config = r.data.config
                }
                $.router.set(window.location.pathname + window.location.search)
            }else if (r.data.logined == 0){ //未注册
                $.router.set("/register")
            }
        })
    stopLoading()
}
$(document).on('click','#btn-login',async function(e){
    loading()
    let username = $('#login-container mdui-text-field[label="username"]').val()
    let password = $('#login-container mdui-text-field[label="password"]').val()
    await api.call(api.login(username,password),
        async (r)=>{
            await initSettingMailAccount()
            if (r.data != null && r.data.config){
                r.data.config.gu = window.origin + "/oauth2/google/callback"
                r.data.config.ou = window.origin + "/oauth2/outlook/callback"
                window.store.setting.config = r.data.config
            }
            $.router.set("/mail?accountId=0&boxName=ALL&pageNum=1")
        })
    stopLoading()
})
$(document).on('click','#btn-register',async function(e){
    let username = $('#register-container mdui-text-field[label="username"]').val()
    let password = $('#register-container mdui-text-field[label="password"]').val()
    let verify = $('#register-container mdui-text-field[label="verify"]').val()
    if (verify != password){
        hint("password not match")
        return
    }
    loading()
    await api.call(api.register(username,password),
        async (r)=>{
            await api.call(api.login(username,password),
                async (r)=>{
                    await initSettingMailAccount()
                    if (r.data != null && r.data.config){
                        r.data.config.gu = window.origin + "/oauth2/google/callback"
                        r.data.config.ou = window.origin + "/oauth2/outlook/callback"
                        window.store.setting.config = r.data.config
                    }
                    $.router.set("/mail?accountId=0&boxName=ALL&pageNum=1")
            })
        })
    stopLoading()
})
init()

$.route('*', (e) => {
    var pathname = window.location.pathname
    var search = window.location.search
    if (search.startsWith("?")){
        search = search.substring(1)
    }
    window.store.searchParams = deparam(search)
    $('.layout-container').hide()
    $('.path-control').hide()
    for (var item of $(`.path-control[path]`)){
        if (pathname.startsWith($(item).attr("path"))){
            $(item).show()
        }
    }
    $("mdui-navigation-drawer").removeAttr('open')
    if (mdui.breakpoint().up("md")){
        $("mdui-navigation-drawer").attr('open',pathname.startsWith("/mail") || pathname.startsWith("/setting"))
    }
    mdui.observeResize(document.body, function(entry, observer) {
        if (mdui.breakpoint().up("md")){
            $("mdui-navigation-drawer").attr('open',pathname.startsWith("/mail") || pathname.startsWith("/setting"))
        }
    })
    if (pathname == "/login"){
        $('#login-container').show()
    }else if (pathname == "/register"){
        $('#register-container').show()
    }else if (pathname.startsWith("/mail")){
        $('#mail-container').show()
        var refreshMailList = false
        if (window.store.searchParams.accountId){
            var oldId = window.store.accountId
            window.store.accountId = window.store.searchParams.accountId
            if (oldId != window.store.accountId){
                refreshMailList = true
                initMailBox()
            }
        }
        var oldSearch = window.store.oldSearch
        var oldBoxName = window.store.boxName
        var oldPageNum = window.store.pageNum
        var oldMailId = window.store.mailId
        if (window.store.searchParams.search){
            window.store.oldSearch = window.store.searchParams.oldSearch
            $("#btn-mail-search").parent().val(window.store.searchParams.search)
        }
        if (window.store.searchParams.boxName){
            window.store.boxName = window.store.searchParams.boxName
        }
        if (window.store.searchParams.pageNum){
            window.store.pageNum = window.store.searchParams.pageNum
        }
        if (oldBoxName != window.store.boxName){
            refreshMailList = true
            $(".mail-box-item").removeAttr("active")
            $(`.mail-box-item[data-n='${window.store.boxName}'`).attr('active','')
        }
        if (oldSearch != window.store.search || oldPageNum != window.store.pageNum){
            refreshMailList = true
        }
        if (refreshMailList){
            initMailList()
        }
        $(".mail-list-item").removeAttr("active")
        if (window.store.searchParams.mailId){
            window.store.mailId = window.store.searchParams.mailId
            initMail()
        }else{
            $("#mail-content").hide()
        }
    }else if (pathname.startsWith("/setting")){
        $('#setting-container').show()
        $("#setting-content>div").hide()
        $(`#setting-content>div[path='${pathname}']`).show()
        initSetting()
        if (pathname == "/setting/webhook"){
            initSettingWebhook()
        }else if (pathname == "/setting/mailAccount"){
            initSettingMailAccount()
        }else if (pathname == "/setting/oauth"){
            initSettingOAuth()
        }else if (pathname == "/setting/login"){
            initSettingLogin()
        }
    }else if (pathname=="/logout"){
        (async ()=>{
            loading()
            await api.call(api.logout(),
            async (r)=>{},
            async (r)=>{})
            stopLoading()
            $.router.set("/login")
        })()
    }else{
        $.router.set("/mail?accountId=0&boxName=ALL&pageNum=1")
    }
});