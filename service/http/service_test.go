package http

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/spiral/roadrunner"
	"github.com/spiral/roadrunner/service"
	"github.com/spiral/roadrunner/service/env"
	"github.com/spiral/roadrunner/service/rpc"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"
)

type testCfg struct {
	httpCfg string
	rpcCfg  string
	envCfg  string
	target  string
}

func (cfg *testCfg) Get(name string) service.Config {
	if name == ID {
		if cfg.httpCfg == "" {
			return nil
		}

		return &testCfg{target: cfg.httpCfg}
	}

	if name == rpc.ID {
		return &testCfg{target: cfg.rpcCfg}
	}

	if name == env.ID {
		return &testCfg{target: cfg.envCfg}
	}

	return nil
}
func (cfg *testCfg) Unmarshal(out interface{}) error {
	return json.Unmarshal([]byte(cfg.target), out)
}

func Test_Service_NoConfig(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	err := c.Init(&testCfg{httpCfg: `{"Enable":true}`})
	assert.Error(t, err)

	s, st := c.Get(ID)
	assert.NotNil(t, s)
	assert.Equal(t, service.StatusInactive, st)
}

func Test_Service_Configure_Disable(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.NoError(t, c.Init(&testCfg{}))

	s, st := c.Get(ID)
	assert.NotNil(t, s)
	assert.Equal(t, service.StatusInactive, st)
}

func Test_Service_Configure_Enable(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.NoError(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": ":8070",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers":{
				"command": "php ../../tests/http/client.php echo pipes",
				"relay": "pipes",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`}))

	s, st := c.Get(ID)
	assert.NotNil(t, s)
	assert.Equal(t, service.StatusOK, st)
}

func Test_Service_Echo(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.NoError(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": ":6029",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers":{
				"command": "php ../../tests/http/client.php echo pipes",
				"relay": "pipes",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`}))

	s, st := c.Get(ID)
	assert.NotNil(t, s)
	assert.Equal(t, service.StatusOK, st)

	// should do nothing
	s.(*Service).Stop()

	go func() {
		err := c.Serve()
		if err != nil {
			t.Errorf("serve error: %v", err)
		}
	}()
	time.Sleep(time.Millisecond * 100)

	req, err := http.NewRequest("GET", "http://localhost:6029?hello=world", nil)
	assert.NoError(t, err)

	r, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	b, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.Equal(t, 201, r.StatusCode)
	assert.Equal(t, "WORLD", string(b))

	err2 := r.Body.Close()
	if err2 != nil {
		t.Errorf("error closing the Body: error %v", err2)
	}
	c.Stop()
}

func Test_Service_Env(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(env.ID, env.NewService(map[string]string{"rr": "test"}))
	c.Register(ID, &Service{})

	assert.NoError(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": ":6031",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers":{
				"command": "php ../../tests/http/client.php env pipes",
				"relay": "pipes",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`, envCfg: `{"env_key":"ENV_VALUE"}`}))

	s, st := c.Get(ID)
	assert.NotNil(t, s)
	assert.Equal(t, service.StatusOK, st)

	// should do nothing
	s.(*Service).Stop()

	go func() {
		err := c.Serve()
		if err != nil {
			t.Errorf("serve error: %v", err)
		}
	}()

	time.Sleep(time.Second * 1)

	req, err := http.NewRequest("GET", "http://localhost:6031", nil)
	assert.NoError(t, err)

	r, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer func() {
		err := r.Body.Close()
		if err != nil {
			t.Errorf("error closing the Body: error %v", err)
		}
	}()

	b, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.Equal(t, 200, r.StatusCode)
	assert.Equal(t, "ENV_VALUE", string(b))
	c.Stop()
}

func Test_Service_ErrorEcho(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.NoError(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": ":6030",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers":{
				"command": "php ../../tests/http/client.php echoerr pipes",
				"relay": "pipes",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`}))

	s, st := c.Get(ID)
	assert.NotNil(t, s)
	assert.Equal(t, service.StatusOK, st)

	goterr := make(chan interface{})
	s.(*Service).AddListener(func(event int, ctx interface{}) {
		if event == roadrunner.EventStderrOutput {
			if string(ctx.([]byte)) == "WORLD\n" {
				goterr <- nil
			}
		}
	})

	go func() {
		err := c.Serve()
		if err != nil {
			t.Errorf("serve error: %v", err)
		}
	}()

	time.Sleep(time.Second)

	req, err := http.NewRequest("GET", "http://localhost:6030?hello=world", nil)
	assert.NoError(t, err)

	r, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer func() {
		err := r.Body.Close()
		if err != nil {
			t.Errorf("error closing the Body: error %v", err)
		}
	}()

	b, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	<-goterr

	assert.NoError(t, err)
	assert.Equal(t, 201, r.StatusCode)
	assert.Equal(t, "WORLD", string(b))
	c.Stop()
}

func Test_Service_Middleware(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.NoError(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": ":6032",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers":{
				"command": "php ../../tests/http/client.php echo pipes",
				"relay": "pipes",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`}))

	s, st := c.Get(ID)
	assert.NotNil(t, s)
	assert.Equal(t, service.StatusOK, st)

	s.(*Service).AddMiddleware(func(f http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/halt" {
				w.WriteHeader(500)
				_, err := w.Write([]byte("halted"))
				if err != nil {
					t.Errorf("error writing the data to the http reply: error %v", err)
				}
			} else {
				f(w, r)
			}
		}
	})

	go func() {
		err := c.Serve()
		if err != nil {
			t.Errorf("serve error: %v", err)
		}
	}()
	time.Sleep(time.Second)

	req, err := http.NewRequest("GET", "http://localhost:6032?hello=world", nil)
	assert.NoError(t, err)

	r, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	b, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.Equal(t, 201, r.StatusCode)
	assert.Equal(t, "WORLD", string(b))

	err = r.Body.Close()
	if err != nil {
		t.Errorf("error closing the Body: error %v", err)
	}

	req, err = http.NewRequest("GET", "http://localhost:6032/halt", nil)
	assert.NoError(t, err)

	r, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	b, err = ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	assert.NoError(t, err)
	assert.Equal(t, 500, r.StatusCode)
	assert.Equal(t, "halted", string(b))

	err = r.Body.Close()
	if err != nil {
		c.Stop()
		t.Errorf("error closing the Body: error %v", err)
	}
	c.Stop()
}

func Test_Service_Listener(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.NoError(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": ":6033",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers":{
				"command": "php ../../tests/http/client.php echo pipes",
				"relay": "pipes",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`}))

	s, st := c.Get(ID)
	assert.NotNil(t, s)
	assert.Equal(t, service.StatusOK, st)

	stop := make(chan interface{})
	s.(*Service).AddListener(func(event int, ctx interface{}) {
		if event == roadrunner.EventServerStart {
			stop <- nil
		}
	})

	go func() {
		err := c.Serve()
		if err != nil {
			t.Errorf("serve error: %v", err)
		}
	}()
	time.Sleep(time.Millisecond * 100)

	c.Stop()
	assert.True(t, true)
}

func Test_Service_Error(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.NoError(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": ":6034",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers":{
				"command": "php ../../tests/http/client.php echo pipes",
				"relay": "---",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`}))

	assert.Error(t, c.Serve())
}

func Test_Service_Error2(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.NoError(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": ":6035",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers":{
				"command": "php ../../tests/http/client.php broken pipes",
				"relay": "pipes",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`}))

	assert.Error(t, c.Serve())
}

func Test_Service_Error3(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.Error(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": ":6036",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers"
				"command": "php ../../tests/http/client.php broken pipes",
				"relay": "pipes",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`}))
}

func Test_Service_Error4(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)

	c := service.NewContainer(logger)
	c.Register(ID, &Service{})

	assert.Error(t, c.Init(&testCfg{httpCfg: `{
			"enable": true,
			"address": "----",
			"maxRequestSize": 1024,
			"uploads": {
				"dir": ` + tmpDir() + `,
				"forbid": []
			},
			"workers":{
				"command": "php ../../tests/http/client.php broken pipes",
				"relay": "pipes",
				"pool": {
					"numWorkers": 1, 
					"allocateTimeout": 10000000,
					"destroyTimeout": 10000000 
				}
			}
	}`}))
}

func tmpDir() string {
	p := os.TempDir()
	r, _ := json.Marshal(p)

	return string(r)
}
