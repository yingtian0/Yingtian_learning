// Package netsim: ループバックの接続に、人工的な遅延 (片道) と、1回きりの停止 (パケット損失の模擬) を加える。
//
// 実際のネットワークでは、往復時間 (RTT) が、プロトコルの手順の回数に、直接効く。
// ループバックは RTT がほぼ 0 なので、そのままでは、その効果が見えない。
// ここでは、書き込んだデータを、一定時間後に、相手に届ける (順序は保つ)。帯域の制限はない。
package netsim

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type item struct {
	at   time.Time
	data []byte
}

// Stall: 配送したバイト数の合計が At を超えたとき、1回だけ、Dur だけ、配送を止める。
// (TCP でパケットが失われ、再送を待つ間、後ろのデータが全部止まることの模擬。)
// 複数の接続で、共有できる (最初に超えた1つの接続だけが、止まる)。
type Stall struct {
	At    int64
	Dur   time.Duration
	fired atomic.Bool
}

type conn struct {
	net.Conn
	delay   time.Duration
	ch      chan item
	done    chan struct{}
	sent    int64
	stall   *Stall
	closeMu sync.Once
}

func Wrap(c net.Conn, oneWay time.Duration, stall *Stall) net.Conn {
	w := &conn{Conn: c, delay: oneWay, ch: make(chan item, 1<<14), done: make(chan struct{}), stall: stall}
	go func() {
		defer close(w.done)
		for it := range w.ch {
			if d := time.Until(it.at); d > 0 {
				time.Sleep(d)
			}
			w.Conn.Write(it.data)
		}
	}()
	return w
}

func (c *conn) Write(b []byte) (int, error) {
	data := append([]byte(nil), b...)
	at := time.Now().Add(c.delay)
	c.sent += int64(len(b))
	if s := c.stall; s != nil && c.sent > s.At && s.fired.CompareAndSwap(false, true) {
		at = at.Add(s.Dur)
	}
	c.ch <- item{at: at, data: data}
	return len(b), nil
}

// 書き込みの期限は、下の接続には、渡さない。
// crypto/tls は、close_notify を送った直後に、「以降の書き込みは失敗させる」ため、書き込みの期限を「今」に設定する。
// 遅延させた書き込みは、後で、下の接続に書くので、その期限に引っかかり、データが失われる (相手には EOF だけが届く)。
func (c *conn) SetWriteDeadline(t time.Time) error { return nil }
func (c *conn) SetDeadline(t time.Time) error      { return c.Conn.SetReadDeadline(t) }

func (c *conn) Close() error {
	var err error
	c.closeMu.Do(func() {
		close(c.ch)
		<-c.done // 溜まっているデータを、届けきってから閉じる
		err = c.Conn.Close()
	})
	return err
}

type Listener struct {
	net.Listener
	Delay time.Duration
	Stall *Stall
}

func (l *Listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return Wrap(c, l.Delay, l.Stall), nil
}
