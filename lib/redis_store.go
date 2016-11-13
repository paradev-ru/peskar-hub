package lib

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
)

type RedisStore struct {
	pool *redis.Pool
}

func NewRedis(maxIdle int, timeoutIdle time.Duration, addr string) *RedisStore {
	var pool = &redis.Pool{
		MaxIdle:     maxIdle,
		IdleTimeout: timeoutIdle,
		Dial: func() (redis.Conn, error) {
			c, err := redis.DialURL(addr)
			if err != nil {
				return nil, err
			}
			if _, err := c.Do("PING"); err != nil {
				c.Close()
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if _, err := c.Do("PING"); err != nil {
				return err
			}
			return nil
		},
	}
	return &RedisStore{pool}
}

func (r *RedisStore) Check() error {
	conn := r.pool.Get()
	defer conn.Close()
	return conn.Err()
}

func (r *RedisStore) Flush() error {
	conn := r.pool.Get()
	defer conn.Close()
	_, err := conn.Do("FLUSHALL")
	return err
}

func (r *RedisStore) Send(channel string, value interface{}) error {
	res, err := json.Marshal(value)
	if err != nil {
		return err
	}

	conn := r.pool.Get()
	defer conn.Close()

	_, err = conn.Do("PUBLISH", channel, string(res))
	return err
}

type Subscribe struct {
	pool *redis.Pool

	Channel                       string
	RetryingPolicyCallback        func(attempts int, duration time.Duration) error
	SuccessReceivedCallback       func(result []byte) error
	ConnectionEstablishedCallback func(duration time.Duration) error
}

func (r *RedisStore) NewSubscribe(channel string) *Subscribe {
	s := new(Subscribe)

	s.pool = r.pool
	s.Channel = channel
	s.RetryingPolicyCallback = basicRetryingPolicyCallback
	s.SuccessReceivedCallback = basicSuccessReveivedCallback
	s.ConnectionEstablishedCallback = basicConnectionEstablishedCallback

	return s
}

func (s *Subscribe) Run() error {
	var attempts int
	var errStartTime time.Time
WAIT:
	for {
		conn := s.pool.Get()
		defer conn.Close()

		psc := redis.PubSubConn{conn}
		err := psc.Subscribe(s.Channel)

		if conn.Err() == nil && err == nil && attempts > 0 {
			attempts = 0
			if err := s.ConnectionEstablishedCallback(time.Since(errStartTime)); err != nil {
				return err
			}
		}

		for {
			switch v := psc.Receive().(type) {
			case redis.Message:
				if err := s.SuccessReceivedCallback(v.Data); err != nil {
					logrus.Error(err)
				}
			case error:
				if attempts == 0 {
					errStartTime = time.Now()
				}

				logrus.Errorf("Redis connection refused: %+v", v)
				psc.Close()

				attempts++

				err := s.RetryingPolicyCallback(attempts, time.Since(errStartTime))
				if err != nil {
					return err
				}

				goto WAIT
			}
		}
	}

	return nil
}

func basicConnectionEstablishedCallback(duration time.Duration) error {
	logrus.Infof("Redis connection established. Downtime: %v", duration)
	return nil
}

func basicRetryingPolicyCallback(attempts int, duration time.Duration) error {
	if duration >= 30*time.Minute {
		return errors.New("Redis connection refused for a 30 minutes, shutting down.")
	}

	logrus.Debugf("Wait Redis for a 10 seconds (#%d, %v)", attempts, duration)
	time.Sleep(10 * time.Second)

	return nil
}

func basicSuccessReveivedCallback(result []byte) error {
	logrus.Infof("Message received: %#v", string(result))
	return nil
}
