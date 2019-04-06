package sync

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	etcdconcurrency "github.com/coreos/etcd/clientv3/concurrency"
)

type Semaphore struct {
	session *etcdconcurrency.Session
	em      *etcdconcurrency.Mutex
	c       int
	m       sync.Mutex
}

func NewSemaphore(session *etcdconcurrency.Session, pfx string) *Semaphore {
	s := &Semaphore{
		session: session,
		em:      etcdconcurrency.NewMutex(session, pfx),
		c:       0,
	}
	return s
}

func (s *Semaphore) TryAcquire(ctx context.Context) error {
	ctx, cancelFunc := context.WithTimeout(ctx, time.Second)
	defer cancelFunc()
	return s.Acquire(ctx)
}

func (s *Semaphore) Acquire(ctx context.Context) error {
	s.m.Lock()
	defer s.m.Unlock()
	if s.c == 0 {
		if err := s.em.Lock(ctx); err != nil {
			return errors.Wrapf(err, "unable to obtain etcd lock")
		}
	}
	s.c++
	return nil
}

func (s *Semaphore) Release(ctx context.Context) error {
	s.m.Lock()
	defer s.m.Unlock()
	s.c--
	if s.c < 0 {
		return errors.New("invalid release of semaphore")
	}
	if s.c == 0 {
		return errors.Wrapf(s.em.Unlock(ctx), "unable to release etcd lock")
	}
	return nil
}
