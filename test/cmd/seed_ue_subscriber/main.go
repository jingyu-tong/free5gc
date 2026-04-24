package main

import (
	"fmt"
	"os"

	"test"

	"github.com/free5gc/util/mongoapi"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

type config struct {
	Supi string `yaml:"supi,omitempty"`
	Mcc  string `yaml:"mcc,omitempty"`
	Mnc  string `yaml:"mnc,omitempty"`
	K    string `yaml:"k,omitempty"`
	Opc  string `yaml:"opc,omitempty"`
	Op   string `yaml:"op,omitempty"`
}

func main() {
	app := cli.NewApp()
	app.Name = "seed-ue-subscriber"
	app.Usage = "Seed subscriber data for ueRanEmulator into MongoDB"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "./ueRanEmulator/config/ueranem.amf-dsmf.yaml",
			Usage:   "Load UE subscriber configuration from `FILE`",
		},
		&cli.StringFlag{
			Name:  "mongo-uri",
			Value: "mongodb://127.0.0.1:27017",
			Usage: "MongoDB connection URI",
		},
		&cli.StringFlag{
			Name:  "mongo-db",
			Value: "free5gc",
			Usage: "MongoDB database name",
		},
	}
	app.Action = action

	if err := app.Run(os.Args); err != nil {
		panic(err)
	}
}

func action(c *cli.Context) error {
	cfg, err := loadConfig(c.String("config"))
	if err != nil {
		return err
	}

	if cfg.Supi == "" || cfg.Mcc == "" || cfg.Mnc == "" || cfg.K == "" || cfg.Opc == "" {
		return fmt.Errorf("supi, mcc, mnc, k and opc are required")
	}
	if err := mongoapi.SetMongoDB(c.String("mongo-db"), c.String("mongo-uri")); err != nil {
		return err
	}

	servingPlmnID := cfg.Mcc + cfg.Mnc
	authSubs := test.GetAuthSubscription(cfg.K, cfg.Opc, cfg.Op)

	_ = test.DelAuthSubscriptionToMongoDB(cfg.Supi)
	_ = test.DelAccessAndMobilitySubscriptionDataFromMongoDB(cfg.Supi, servingPlmnID)
	_ = test.DelSessionManagementSubscriptionDataFromMongoDB(cfg.Supi, servingPlmnID)
	_ = test.DelSmfSelectionSubscriptionDataFromMongoDB(cfg.Supi, servingPlmnID)
	_ = test.DelAmPolicyDataFromMongoDB(cfg.Supi)
	_ = test.DelSmPolicyDataFromMongoDB(cfg.Supi)
	_ = test.DelChargingDataFromMongoDB(cfg.Supi, servingPlmnID)
	_ = test.DelFlowRuleFromMongoDB(cfg.Supi, servingPlmnID)
	_ = test.DelQosFlowFromMongoDB(cfg.Supi, servingPlmnID)

	test.InsertAuthSubscriptionToMongoDB(cfg.Supi, authSubs)
	test.InsertWebAuthSubscriptionToMongoDB(cfg.Supi, authSubs)
	test.InsertAccessAndMobilitySubscriptionDataToMongoDB(cfg.Supi, test.GetAccessAndMobilitySubscriptionData(), servingPlmnID)
	test.InsertSmfSelectionSubscriptionDataToMongoDB(cfg.Supi, test.GetSmfSelectionSubscriptionData(), servingPlmnID)
	test.InsertSessionManagementSubscriptionDataToMongoDB(cfg.Supi, servingPlmnID, test.GetSessionManagementSubscriptionData())
	test.InsertAmPolicyDataToMongoDB(cfg.Supi, test.GetAmPolicyData())
	test.InsertSmPolicyDataToMongoDB(cfg.Supi, test.GetSmPolicyData())
	test.InsertChargingDataToMongoDB(cfg.Supi, servingPlmnID, test.GetChargingData())
	test.InsertFlowRuleToMongoDB(cfg.Supi, servingPlmnID, test.GetFlowRuleData())
	test.InsertQoSFlowToMongoDB(cfg.Supi, servingPlmnID, test.GetQosFlowData())

	fmt.Printf("Seeded subscriber %s for PLMN %s\n", cfg.Supi, servingPlmnID)
	return nil
}

func loadConfig(path string) (*config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &config{}
	if err := yaml.Unmarshal(content, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
