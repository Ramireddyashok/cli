package v2

import (
	"os"

	"github.com/cloudfoundry/noaa/consumer"

	"code.cloudfoundry.org/cli/actor/sharedaction"
	"code.cloudfoundry.org/cli/actor/v2action"
	"code.cloudfoundry.org/cli/actor/v3action"
	oldCmd "code.cloudfoundry.org/cli/cf/cmd"
	"code.cloudfoundry.org/cli/command"
	"code.cloudfoundry.org/cli/command/flag"
	"code.cloudfoundry.org/cli/command/v2/shared"
	sharedV3 "code.cloudfoundry.org/cli/command/v3/shared"
)

//go:generate counterfeiter . StartActor

type StartActor interface {
	GetApplicationByNameAndSpace(name string, spaceGUID string) (v2action.Application, v2action.Warnings, error)
	GetApplicationSummaryByNameAndSpace(name string, spaceGUID string) (v2action.ApplicationSummary, v2action.Warnings, error)
	StartApplication(app v2action.Application, client v2action.NOAAClient, config v2action.Config) (<-chan *v2action.LogMessage, <-chan error, <-chan bool, <-chan string, <-chan error)
}

//go:generate counterfeiter . StartActorV3

type StartActorV3 interface {
	CloudControllerAPIVersion() string
}

type StartCommand struct {
	RequiredArgs        flag.AppName `positional-args:"yes"`
	usage               interface{}  `usage:"CF_NAME start APP_NAME"`
	envCFStagingTimeout interface{}  `environmentName:"CF_STAGING_TIMEOUT" environmentDescription:"Max wait time for buildpack staging, in minutes" environmentDefault:"15"`
	envCFStartupTimeout interface{}  `environmentName:"CF_STARTUP_TIMEOUT" environmentDescription:"Max wait time for app instance startup, in minutes" environmentDefault:"5"`
	relatedCommands     interface{}  `related_commands:"apps, logs, scale, ssh, stop, restart, run-task"`

	UI          command.UI
	Config      command.Config
	SharedActor command.SharedActor
	Actor       StartActor
	ActorV3     StartActorV3
	NOAAClient  *consumer.Consumer
}

func (cmd *StartCommand) Setup(config command.Config, ui command.UI) error {
	cmd.UI = ui
	cmd.Config = config
	cmd.SharedActor = sharedaction.NewActor()

	ccClient, uaaClient, err := shared.NewClients(config, ui, true)
	if err != nil {
		return err
	}
	cmd.Actor = v2action.NewActor(ccClient, uaaClient)

	ccClientV3, err := sharedV3.NewClients(config, ui, true)
	if err != nil {
		return err
	}
	cmd.ActorV3 = v3action.NewActor(ccClientV3)

	cmd.NOAAClient = shared.NewNOAAClient(ccClient.DopplerEndpoint(), config, uaaClient, ui)

	return nil
}

func (cmd StartCommand) Execute(args []string) error {
	if cmd.Config.Experimental() == false {
		oldCmd.Main(os.Getenv("CF_TRACE"), os.Args)
		return nil
	}

	cmd.UI.DisplayText(command.ExperimentalWarning)
	cmd.UI.DisplayNewline()

	err := cmd.SharedActor.CheckTarget(cmd.Config, true, true)
	if err != nil {
		return shared.HandleError(err)
	}

	user, err := cmd.Config.CurrentUser()
	if err != nil {
		return shared.HandleError(err)
	}

	cmd.UI.DisplayTextWithFlavor("Starting app {{.AppName}} in org {{.OrgName}} / space {{.SpaceName}} as {{.CurrentUser}}...",
		map[string]interface{}{
			"AppName":     cmd.RequiredArgs.AppName,
			"OrgName":     cmd.Config.TargetedOrganization().Name,
			"SpaceName":   cmd.Config.TargetedSpace().Name,
			"CurrentUser": user.Name,
		})

	app, warnings, err := cmd.Actor.GetApplicationByNameAndSpace(cmd.RequiredArgs.AppName, cmd.Config.TargetedSpace().GUID)
	cmd.UI.DisplayWarnings(warnings)
	if err != nil {
		return shared.HandleError(err)
	}

	if app.Started() {
		cmd.UI.DisplayText("App {{.AppName}} is already started",
			map[string]interface{}{
				"AppName": cmd.RequiredArgs.AppName,
			})
		return nil
	}

	messages, logErrs, appStarting, apiWarnings, errs := cmd.Actor.StartApplication(app, cmd.NOAAClient, cmd.Config)
	cmd.UI.DisplayNewline()
	err = shared.PollStart(cmd.UI, cmd.Config, messages, logErrs, appStarting, apiWarnings, errs)
	if err != nil {
		return err
	}

	cmd.UI.DisplayNewline()

	appSummary, warnings, err := cmd.Actor.GetApplicationSummaryByNameAndSpace(cmd.RequiredArgs.AppName, cmd.Config.TargetedSpace().GUID)
	cmd.UI.DisplayWarnings(warnings)
	if err != nil {
		return shared.HandleError(err)
	}

	displayIsolationSegment := command.MinimumAPIVersionCheck(cmd.ActorV3.CloudControllerAPIVersion(), "3.11.0") == nil

	shared.DisplayAppSummary(cmd.UI, appSummary, true, displayIsolationSegment)
	return nil
}
