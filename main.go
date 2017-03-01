package main

import (
	"os"

	"github.com/Dataman-Cloud/swan/src/config"
	"github.com/Dataman-Cloud/swan/src/node"
	"github.com/Dataman-Cloud/swan/src/version"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

func setupLogger(logLevel string) {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.DebugLevel
	}
	logrus.SetLevel(level)

	logrus.SetOutput(os.Stderr)

	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	})
}

func main() {
	app := cli.NewApp()
	app.Name = "swan"
	app.Usage = "swan [ROLE] [COMMAND] [ARG...]"
	app.Description = "A general purpose Mesos framework which facility long running docker application management."
	app.Version = version.Version

	app.Commands = []cli.Command{}

	app.Commands = append(app.Commands, AgentJoinCmd())
	app.Commands = append(app.Commands, ManagerCmd())

	if err := app.Run(os.Args); err != nil {
		logrus.Errorf("%s", err.Error())
		os.Exit(1)
	}
}

func FlagListenAddr() cli.Flag {
	return cli.StringFlag{
		Name:   "listen-addr",
		Usage:  "listener address for agent",
		EnvVar: "SWAN_LISTEN_ADDR",
		Value:  "0.0.0.0:9999",
	}
}

func FlagAdvertiseAddr() cli.Flag {
	return cli.StringFlag{
		Name:   "advertise-addr",
		Usage:  "advertise address for agent, default is the listen-addr",
		EnvVar: "SWAN_ADVERTISE_ADDR",
		Value:  "",
	}
}

func FlagRaftListenAddr() cli.Flag {
	return cli.StringFlag{
		Name:   "raft-listen-addr",
		Usage:  "swan raft serverlistener address",
		EnvVar: "SWAN_RAFT_LISTEN_ADDR",
		Value:  "http://0.0.0.0:2111",
	}
}

func FlagRaftAdvertiseAddr() cli.Flag {
	return cli.StringFlag{
		Name:   "raft-advertise-addr",
		Usage:  "swan raft advertise address, default is the raft-listen-addr",
		EnvVar: "SWAN_RAFT_ADVERTISE_ADDR",
		Value:  "",
	}
}

func FlagJoinAddrs() cli.Flag {
	return cli.StringFlag{
		Name:   "join-addrs",
		Usage:  "the addrs new node join to. Splited by ','",
		EnvVar: "SWAN_JOIN_ADDRS",
		Value:  "0.0.0.0:9999",
	}
}

func FlagJanitorAdvertiseIp() cli.Flag {
	return cli.StringFlag{
		Name:   "janitor-advertise-ip",
		Usage:  "janitor proxy advertise ip",
		EnvVar: "SWAN_JANITOR_ADVERTISE_IP",
		Value:  "",
	}
}

func FlagZkPath() cli.Flag {
	return cli.StringFlag{
		Name:   "zk-path",
		Usage:  "zookeeper mesos paths. eg. zk://host1:port1,host2:port2,.../path",
		EnvVar: "SWAN_MESOS_ZKPATH",
		Value:  "localhost:2181/mesos",
	}
}

func FlagLogLevel() cli.Flag {
	return cli.StringFlag{
		Name:   "log-level,l",
		Usage:  "customize log level [debug|info|error]",
		EnvVar: "SWAN_LOG_LEVEL",
		Value:  "info",
	}
}

func FlagDataDir() cli.Flag {
	return cli.StringFlag{
		Name:   "data-dir,d",
		Usage:  "swan data store dir",
		EnvVar: "SWAN_DATA_DIR",
		Value:  "./data",
	}
}

func FlagDomain() cli.Flag {
	return cli.StringFlag{
		Name:   "domain",
		Usage:  "domain which resolve to proxies. eg. swan.com, which make any task can be access from path likes 0.appname.username.cluster.swan.com",
		EnvVar: "SWAN_DOMAIN",
		Value:  "swan.com",
	}
}

func AgentJoinCmd() cli.Command {
	agentJoinCmd := cli.Command{
		Name:        "agent",
		Usage:       "[COMMAND] [ARG...]",
		Description: "start and join a swan agent which contains proxy and DNS server",
		Flags:       []cli.Flag{},
		Action:      JoinAndStartAgent,
	}

	agentJoinCmd.Flags = append(agentJoinCmd.Flags, FlagListenAddr())
	agentJoinCmd.Flags = append(agentJoinCmd.Flags, FlagAdvertiseAddr())
	agentJoinCmd.Flags = append(agentJoinCmd.Flags, FlagJoinAddrs())
	agentJoinCmd.Flags = append(agentJoinCmd.Flags, FlagJanitorAdvertiseIp())
	agentJoinCmd.Flags = append(agentJoinCmd.Flags, FlagLogLevel())
	agentJoinCmd.Flags = append(agentJoinCmd.Flags, FlagDomain())

	return agentJoinCmd
}

func JoinAndStartAgent(c *cli.Context) error {
	conf, err := config.NewConfig(c)
	conf.Mode = config.Agent
	if err != nil {
		logrus.Errorf("load config failed. Error: %s", err)
		return err
	}

	setupLogger(conf.LogLevel)

	node, err := node.NewNode(conf)
	if err != nil {
		logrus.Error("Node initialization failed")
		return err
	}

	if err := node.Start(context.Background()); err != nil {
		logrus.Errorf("start node failed. Error: %s", err.Error())
		return err
	}

	return nil
}

func ManagerCmd() cli.Command {
	managerCmd := cli.Command{
		Name:        "manager",
		Usage:       "[COMMAND] [ARG...]",
		Description: "init a manager as new cluster or join to an exiting cluster",
		Subcommands: []cli.Command{},
	}

	managerCmd.Subcommands = append(managerCmd.Subcommands, ManagerJoinCmd())
	managerCmd.Subcommands = append(managerCmd.Subcommands, ManagerInitCmd())

	return managerCmd
}

func ManagerJoinCmd() cli.Command {
	managerJoinCmd := cli.Command{
		Name:        "join",
		Usage:       "join [ARG...]",
		Description: "start a manager and join to an exitsing swan cluster",
		Flags:       []cli.Flag{},
		Action:      JoinAndStartManager,
	}

	managerJoinCmd.Flags = append(managerJoinCmd.Flags, FlagListenAddr())
	managerJoinCmd.Flags = append(managerJoinCmd.Flags, FlagAdvertiseAddr())
	managerJoinCmd.Flags = append(managerJoinCmd.Flags, FlagRaftListenAddr())
	managerJoinCmd.Flags = append(managerJoinCmd.Flags, FlagRaftAdvertiseAddr())
	managerJoinCmd.Flags = append(managerJoinCmd.Flags, FlagJoinAddrs())
	managerJoinCmd.Flags = append(managerJoinCmd.Flags, FlagZkPath())
	managerJoinCmd.Flags = append(managerJoinCmd.Flags, FlagLogLevel())
	managerJoinCmd.Flags = append(managerJoinCmd.Flags, FlagDataDir())

	return managerJoinCmd
}

func JoinAndStartManager(c *cli.Context) error {
	conf, err := config.NewConfig(c)
	conf.Mode = config.Manager
	if err != nil {
		logrus.Errorf("load config failed. Error: %s", err)
		return err
	}

	setupLogger(conf.LogLevel)

	node, err := node.NewNode(conf)
	if err != nil {
		logrus.Error("Node initialization failed")
		return err
	}

	if err := node.Start(context.Background()); err != nil {
		logrus.Errorf("start node failed. Error: %s", err.Error())
		return err
	}

	return nil
}

func ManagerInitCmd() cli.Command {
	managerInitCmd := cli.Command{
		Name:        "init",
		Usage:       "init [ARG...]",
		Description: "start a manager and init a new swan cluster",
		Flags:       []cli.Flag{},
		Action:      StartManager,
	}

	managerInitCmd.Flags = append(managerInitCmd.Flags, FlagListenAddr())
	managerInitCmd.Flags = append(managerInitCmd.Flags, FlagAdvertiseAddr())
	managerInitCmd.Flags = append(managerInitCmd.Flags, FlagRaftListenAddr())
	managerInitCmd.Flags = append(managerInitCmd.Flags, FlagRaftAdvertiseAddr())
	managerInitCmd.Flags = append(managerInitCmd.Flags, FlagZkPath())
	managerInitCmd.Flags = append(managerInitCmd.Flags, FlagLogLevel())
	managerInitCmd.Flags = append(managerInitCmd.Flags, FlagDataDir())

	return managerInitCmd
}

func StartManager(c *cli.Context) error {
	conf, err := config.NewConfig(c)
	conf.Mode = config.Manager
	if err != nil {
		logrus.Errorf("load config failed. Error: %s", err)
		return err
	}

	setupLogger(conf.LogLevel)

	node, err := node.NewNode(conf)
	if err != nil {
		logrus.Error("Node initialization failed")
		return err
	}

	if err := node.Start(context.Background()); err != nil {
		logrus.Errorf("start node failed. Error: %s", err.Error())
		return err
	}

	return nil
}
