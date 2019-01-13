package main // import "github.com/whefter/watchtower"

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"strconv"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/whefter/watchtower/actions"
	"github.com/whefter/watchtower/container"
	"github.com/whefter/watchtower/notifications"
)

// DockerAPIMinVersion is the version of the docker API, which is minimally required by
// watchtower. Currently we require at least API 1.24 and therefore Docker 1.12 or later.
const DockerAPIMinVersion string = "1.24"

var version = "master"
var commit = "unknown"
var date = "unknown"

var (
	client       container.Client
	scheduleSpec string
	cleanup      bool
	noRestart    bool
	tag          string
	notifier     *notifications.Notifier
	timeout      time.Duration
)

func init() {
	log.SetLevel(log.InfoLevel)
}

func main() {
	app := cli.NewApp()
	app.Name = "watchtower"
	app.Version = version + " - " + commit + " - " + date
	app.Usage = "Automatically update running Docker containers"
	app.Before = before
	app.Action = start
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Usage:  "daemon socket to connect to",
			Value:  "unix:///var/run/docker.sock",
			EnvVar: "DOCKER_HOST",
		},
		cli.IntFlag{
			Name:   "interval, i",
			Usage:  "poll interval (in seconds)",
			Value:  300,
			EnvVar: "WATCHTOWER_POLL_INTERVAL",
		},
		cli.StringFlag{
			Name:   "schedule, s",
			Usage:  "the cron expression which defines when to update",
			EnvVar: "WATCHTOWER_SCHEDULE",
		},
		cli.BoolFlag{
			Name:   "no-pull",
			Usage:  "do not pull new images",
			EnvVar: "WATCHTOWER_NO_PULL",
		},
		cli.BoolFlag{
			Name:   "no-restart",
			Usage:  "do not restart containers",
			EnvVar: "WATCHTOWER_NO_RESTART",
		},
		cli.BoolFlag{
			Name:   "cleanup",
			Usage:  "remove old images after updating",
			EnvVar: "WATCHTOWER_CLEANUP",
		},
		cli.BoolFlag{
			Name:   "tlsverify",
			Usage:  "use TLS and verify the remote",
			EnvVar: "DOCKER_TLS_VERIFY",
		},
		cli.DurationFlag{
			Name:   "stop-timeout",
			Usage:  "timeout before container is forcefully stopped",
			Value:  time.Second * 10,
			EnvVar: "WATCHTOWER_TIMEOUT",
		},
		cli.StringFlag{
			Name:   "tag",
			Usage:  "Watch containers with this value of the de.whefter.watchtower.tag label",
			EnvVar: "WATCHTOWER_TAG",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug mode with verbose logging",
		},
		cli.StringSliceFlag{
			Name:   "notifications",
			Value:  &cli.StringSlice{},
			Usage:  "notification types to send (valid: email, slack, msteams)",
			EnvVar: "WATCHTOWER_NOTIFICATIONS",
		},
		cli.StringFlag{
			Name:   "notifications-level",
			Usage:  "The log level used for sending notifications. Possible values: \"panic\", \"fatal\", \"error\", \"warn\", \"info\" or \"debug\"",
			EnvVar: "WATCHTOWER_NOTIFICATIONS_LEVEL",
			Value:  "info",
		},
		cli.StringFlag{
			Name:   "notification-email-from",
			Usage:  "Address to send notification e-mails from",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_FROM",
		},
		cli.StringFlag{
			Name:   "notification-email-to",
			Usage:  "Address to send notification e-mails to",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_TO",
		},
		cli.StringFlag{
			Name:   "notification-email-server",
			Usage:  "SMTP server to send notification e-mails through",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER",
		},
		cli.IntFlag{
			Name:   "notification-email-server-port",
			Usage:  "SMTP server port to send notification e-mails through",
			Value:  25,
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT",
		},
		cli.BoolFlag{
			Name: "notification-email-server-tls-skip-verify",
			Usage: "Controls whether watchtower verifies the SMTP server's certificate chain and host name. " +
				"If set, TLS accepts any certificate " +
				"presented by the server and any host name in that certificate. " +
				"In this mode, TLS is susceptible to man-in-the-middle attacks. " +
				"This should be used only for testing.",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY",
		},
		cli.StringFlag{
			Name:   "notification-email-server-user",
			Usage:  "SMTP server user for sending notifications",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER",
		},
		cli.StringFlag{
			Name:   "notification-email-server-password",
			Usage:  "SMTP server password for sending notifications",
			EnvVar: "WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD",
		},
		cli.StringFlag{
			Name:   "notification-slack-hook-url",
			Usage:  "The Slack Hook URL to send notifications to",
			EnvVar: "WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL",
		},
		cli.StringFlag{
			Name:   "notification-slack-identifier",
			Usage:  "A string which will be used to identify the messages coming from this watchtower instance. Default if omitted is \"watchtower\"",
			EnvVar: "WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER",
			Value:  "watchtower",
		},
		cli.StringFlag{
			Name:   "notification-msteams-hook",
			Usage:  "The MSTeams WebHook URL to send notifications to",
			EnvVar: "WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL",
		},
		cli.BoolFlag{
			Name:   "notification-msteams-data",
			Usage:  "The MSTeams notifier will try to extract log entry fields as MSTeams message facts",
			EnvVar: "WATCHTOWER_NOTIFICATION_MSTEAMS_USE_LOG_DATA",
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func before(c *cli.Context) error {
	if c.GlobalBool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	pollingSet := c.IsSet("interval")
	cronSet := c.IsSet("schedule")

	if pollingSet && cronSet {
		log.Fatal("Only schedule or interval can be defined, not both.")
	} else if cronSet {
		scheduleSpec = c.String("schedule")
	} else {
		scheduleSpec = "@every " + strconv.Itoa(c.Int("interval")) + "s"
	}

	cleanup = c.GlobalBool("cleanup")
	noRestart = c.GlobalBool("no-restart")
	timeout = c.GlobalDuration("stop-timeout")
	if timeout < 0 {
		log.Fatal("Please specify a positive value for timeout value.")
	}

	tag = c.String("tag")
	if len(tag) == 0 {
		log.Fatal("Please specify a tag to check for on other containers")
	}

	// configure environment vars for client
	err := envConfig(c)
	if err != nil {
		return err
	}

	client = container.NewClient(!c.GlobalBool("no-pull"))
	notifier = notifications.NewNotifier(c)

	return nil
}

func start(c *cli.Context) error {
	if err := actions.CheckPrereqs(client, tag, cleanup); err != nil {
		log.Fatal(err)
	}

	tagFilter := container.BuildTagFilter(tag)

	tryLockSem := make(chan bool, 1)
	tryLockSem <- true

	cron := cron.New()
	err := cron.AddFunc(
		scheduleSpec,
		func() {
			select {
			case v := <-tryLockSem:
				defer func() { tryLockSem <- v }()
				notifier.StartNotification()
				if err := actions.Update(client, tagFilter, cleanup, noRestart, timeout); err != nil {
					log.Println(err)
				}
				notifier.SendNotification()
			default:
				log.Debug("Skipped another update already running.")
			}

			nextRuns := cron.Entries()
			if len(nextRuns) > 0 {
				log.Debug("Scheduled next run: " + nextRuns[0].Next.String())
			}
		})

	if err != nil {
		return err
	}

	log.Info("Watching container tag: " + tag)
	log.Info("First run: " + cron.Entries()[0].Schedule.Next(time.Now()).String())
	cron.Start()

	// Graceful shut-down on SIGINT/SIGTERM
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	signal.Notify(interrupt, syscall.SIGTERM)

	<-interrupt
	cron.Stop()
	log.Info("Waiting for running update to be finished...")
	<-tryLockSem
	os.Exit(1)
	return nil
}

func setEnvOptStr(env string, opt string) error {
	if opt != "" && opt != os.Getenv(env) {
		err := os.Setenv(env, opt)
		if err != nil {
			return err
		}
	}
	return nil
}

func setEnvOptBool(env string, opt bool) error {
	if opt == true {
		return setEnvOptStr(env, "1")
	}
	return nil
}

// envConfig translates the command-line options into environment variables
// that will initialize the api client
func envConfig(c *cli.Context) error {
	var err error

	err = setEnvOptStr("DOCKER_HOST", c.GlobalString("host"))
	err = setEnvOptBool("DOCKER_TLS_VERIFY", c.GlobalBool("tlsverify"))
	err = setEnvOptStr("DOCKER_API_VERSION", DockerAPIMinVersion)

	return err
}
