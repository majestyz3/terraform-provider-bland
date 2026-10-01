package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/majestyz3/terraform-provider-bland/internal/provider"
)

var version = "dev"

func main() {
	opts := providerserver.ServeOpts{Address: "registry.terraform.io/majestyz3/bland"}
	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err)
	}
}
