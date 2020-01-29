package session

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// 设置、获取、删除 session，返回 sessionID
type Session interface {
	Set(key, value interface{}) error
	Get(key interface{}) interface{}
	Delete(key interface{}) error
	SessionID() string
}

// session 是保存在服务器端的数据，可以以任何方式存储，比如存储在内存、数据库或者文件中。
// 因此抽象出一个 Provider 接口，用以表征 session 管理器底层存储结构
type Provider interface {
	SessionInit(sid string) (Session, error) // 初始化
	SessionRead(sid string) (Session, error) // 读取
	SessionDestroy(sid string) error         // 销毁
	SessionGC(maxLifeTime int64)             // 根据 maxLifeTime 来删除过期的数据
}

var providers = make(map[string]Provider)

// 注册 session 管理器 provider
func Register(name string, provider Provider) {
	if provider == nil {
		panic("session: Register provider is nil")
	}

	if _, dup := providers[name]; dup {
		panic("session: Register called twice for provider " + name)
	}

	providers[name] = provider
}

// 全局 session 管理器
type Manager struct {
	cookieName  string
	lock        sync.Mutex
	provider    Provider
	maxLifeTime int64
}

func NewManager(provideName, cookieName string, maxLifeTime int64) (*Manager, error) {
	provider, ok := providers[provideName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provide %q (forgotten import?)", provideName)
	}
	return &Manager{provider: provider, cookieName: cookieName, maxLifeTime: maxLifeTime}, nil
}

// 生成 sessionID
func (manager *Manager) sessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// 生成 session，并把 sessionID 传送给客户端，sessionID 其实就是 cookie 的值
func (manager *Manager) SessionStart(w http.ResponseWriter, r *http.Request) (session Session) {
	manager.lock.Lock()
	defer manager.lock.Unlock()
	cookie, err := r.Cookie(manager.cookieName)
	// cookie 中是否已经存在 sessionID 了
	if err != nil || cookie.Value == "" {
		sid := manager.sessionID()
		session, err = manager.provider.SessionInit(sid)

		cookie := http.Cookie{
			Name:     manager.cookieName,
			Value:    url.QueryEscape(sid),
			Path:     "/",
			HttpOnly: true,
			MaxAge:   int(manager.maxLifeTime)}
		http.SetCookie(w, &cookie)
	} else {
		sid, _ := url.QueryUnescape(cookie.Value)
		session, _ = manager.provider.SessionRead(sid)
	}

	return
}

// 销毁 session，并通过响应头部 Set-Cookie 对 cookie 进行过期时间设置达到销毁 cookie 的目的
func (manager *Manager) SessionDestroy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(manager.cookieName)
	// 是否需要对 cookie 进行处理
	if err != nil || cookie.Value == "" {
		return
	} else {
		manager.lock.Lock()
		defer manager.lock.Unlock()
		manager.provider.SessionDestroy(cookie.Value)
		expiration := time.Now()
		cookie := http.Cookie{Name: manager.cookieName, Path: "/", HttpOnly: true, Expires: expiration, MaxAge: -1}
		http.SetCookie(w, &cookie)
	}
}

// 销毁
func (manager *Manager) GC() {
	manager.lock.Lock()
	defer manager.lock.Unlock()
	manager.provider.SessionGC(manager.maxLifeTime)

	// 利用 time 包中的定时器功能，当超时 maxLifeTime 之后调用 GC 函数，
	// 这样就可以保证 maxLifeTime 时间内的 session 都是可用的，
	// 类似的方案也可以用于统计在线用户数之类的。
	time.AfterFunc(time.Duration(manager.maxLifeTime), func() {
		manager.GC()
	})
}
