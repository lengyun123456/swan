package agent

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"

	"github.com/Dataman-Cloud/swan/agent/janitor"
	"github.com/Dataman-Cloud/swan/agent/resolver"
	"github.com/Dataman-Cloud/swan/config"
	"github.com/Dataman-Cloud/swan/mole"
)

type Agent struct {
	config      config.AgentConfig
	resolver    *resolver.Resolver
	janitor     *janitor.JanitorServer
	clusterNode *mole.Agent
}

func New(cfg config.AgentConfig) *Agent {
	agent := &Agent{
		config:   cfg,
		resolver: resolver.NewResolver(&cfg.DNS, cfg.Janitor.AdvertiseIP),
		janitor:  janitor.NewJanitorServer(&cfg.Janitor),
	}
	return agent
}

func (agent *Agent) StartAndJoin() error {
	// detect healhty master firstly
	addr, err := agent.detectManagerAddr()
	if err != nil {
		return err
	}

	// sync all of dns & proxy records on start up
	if err := agent.syncFull(addr); err != nil {
		return fmt.Errorf("full sync manager's records error: %v", err)
	}

	// startup resolver & janitor
	go func() {
		if err := agent.resolver.Start(); err != nil {
			log.Fatalln("resolver occured fatal error:", err)
		}
	}()

	go func() {
		if err := agent.janitor.Start(); err != nil {
			log.Fatalln("janitor occured fatal error:", err)
		}
	}()

	// serving protocol & Api with underlying mole
	var (
		delayMin = time.Second      // min retry delay 1s
		delayMax = time.Second * 60 // max retry delay 60s
		delay    = delayMin         // retry delay
	)
	for {
		err := agent.Join()
		if err != nil {
			log.Errorln("agent Join() error:", err)
			delay *= 2
			if delay > delayMax {
				delay = delayMax // reset delay to max
			}
			log.Warnln("agent ReJoin in", delay.String())
			time.Sleep(delay)
			continue
		}

		l := agent.NewListener()

		go func(l net.Listener) {
			err := agent.ServeProtocol()
			if err != nil {
				log.Errorln("agent ServeProtocol() error:", err)
				l.Close() // close the listener -> the ServeApi() return with error -> Rejoin triggered.
			}
		}(l)

		log.Println("agent Joined succeed, ready ...")
		delay = delayMin // reset dealy to min
		err = agent.ServeApi(l)
		if err != nil {
			log.Errorln("agent ServeApi() error:", err)
		}
	}

	return nil
}

func (agent *Agent) Join() error {
	// detect healhty master
	addr, err := agent.detectManagerAddr()
	if err != nil {
		return err
	}
	masterURL, err := url.Parse(addr)
	if err != nil {
		return err
	}

	// setup & join
	agent.clusterNode = mole.NewAgent(&mole.Config{
		Role:   mole.RoleAgent,
		Master: masterURL,
	})

	return agent.clusterNode.Join()
}

func (agent *Agent) NewListener() net.Listener {
	return agent.clusterNode.NewListener()
}

func (agent *Agent) ServeProtocol() error {
	return agent.clusterNode.ServeProtocol()
}

func (agent *Agent) ServeApi(l net.Listener) error {
	log.Println("agent api in serving ...")

	httpd := &http.Server{
		Handler: agent.NewHTTPMux(),
	}
	return httpd.Serve(l)
}

func (agent *Agent) sysinfo(ctx *gin.Context) {
	info, err := Gather()
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx.JSON(200, info)
}

func (agent *Agent) serveProxy(ctx *gin.Context) {
	var (
		r = ctx.Request
		w = ctx.Writer
	)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		w.WriteHeader(500)
		return
	}

	connMaster, _, err := hijacker.Hijack()
	if err != nil {
		w.WriteHeader(500)
		return
	}
	defer connMaster.Close()

	connBackend, err := agent.dialBackend()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer connBackend.Close()

	go func() {
		r.Write(connBackend)
	}()

	io.Copy(connMaster, connBackend)
}

func (agent *Agent) dialBackend() (net.Conn, error) {
	return net.Dial("unix", "/var/run/docker.sock")
}

func (agent *Agent) detectManagerAddr() (string, error) {
	for _, addr := range agent.config.JoinAddrs {
		resp, err := http.Get("http://" + addr + "/ping")
		if err != nil {
			log.Warnf("detect swan manager %s error %v", addr, err)
			continue
		}
		resp.Body.Close() // prevent fd leak

		log.Infof("detect swan manager %s succeed", addr)
		return "http://" + addr, nil
	}

	return "", errors.New("all of swan manager unavailable")
}
