// awsc is a full-screen terminal UI for AWS, inspired by K9s.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tpriestnall/awsc/internal/config"
	"github.com/tpriestnall/awsc/internal/ui"
	ec2view "github.com/tpriestnall/awsc/internal/ui/views/ec2"
	ecrview "github.com/tpriestnall/awsc/internal/ui/views/ecr"
	servicesview "github.com/tpriestnall/awsc/internal/ui/views/services"
	sgview "github.com/tpriestnall/awsc/internal/ui/views/sg"
	subnetview "github.com/tpriestnall/awsc/internal/ui/views/subnet"
	vpcview "github.com/tpriestnall/awsc/internal/ui/views/vpc"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	profileFlag := flag.String("profile", "", "AWS profile to use (default: AWS_PROFILE env or 'default')")
	regionFlag := flag.String("region", "", "AWS region to use (default: AWS_REGION env or 'us-east-1')")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("awsc %s (%s)\n", version, commit)
		os.Exit(0)
	}

	cfg := config.NewAppConfig(*profileFlag, *regionFlag)

	app, err := ui.NewApp(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Register views
	app.RegisterView(servicesview.NewView(app))
	app.RegisterView(ec2view.NewListView(app))
	app.RegisterView(ecrview.NewListView(app))
	app.RegisterView(sgview.NewListView(app))
	app.RegisterView(vpcview.NewListView(app))
	app.RegisterView(subnetview.NewListView(app, ""))

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
