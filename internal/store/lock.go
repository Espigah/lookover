package store

import (
	"os"
	"path/filepath"
	"syscall"

	"lookover/internal/paths"
)

// Lock serializa as escritas da PRÓPRIA sessão (read-modify-write do digest),
// evitando perda de evento quando dois hooks da mesma sessão se sobrepõem.
// Não há contenção cross-sessão: cada sessão trava só o seu próprio arquivo.
type Lock struct{ f *os.File }

// Acquire pega um flock exclusivo do arquivo de lock da sessão.
func Acquire(sessionID string) (*Lock, error) {
	if err := paths.EnsureStore(); err != nil {
		return nil, err
	}
	p := filepath.Join(paths.StoreDir(), sessionID+".lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

// Release solta o lock.
func (l *Lock) Release() {
	if l == nil || l.f == nil {
		return
	}
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	l.f.Close()
}
