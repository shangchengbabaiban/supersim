package main

import (
	"fmt"
	"os"
	"path"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/artifacts"
	"github.com/ethereum-optimism/optimism/op-service/ioutil"
	"github.com/ethereum-optimism/optimism/op-service/jsonutil"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum-optimism/supersim/genesis/worldgen"
	"github.com/urfave/cli/v2"
)

func main() {
	oplog.SetupDefaults()

	app := cli.NewApp()
	app.Name = "supersim-worldgen"
	app.Usage = "Supersim Genesis Generator"
	app.Description = "Generate genesis files for supersim"
	app.Action = worldgenMain
	app.Flags = worldgen.CLIFlags()

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
	}
}

func worldgenMain(ctx *cli.Context) error {
	logger := oplog.NewLogger(oplog.AppOut(nil), oplog.DefaultCLIConfig())

	cliConfig, err := worldgen.ParseCLIConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to parse cli config: %w", err)
	}

	// Download monorepo contract artifacts
	monorepoProgressor := func(curr, total int64) {
		logger.Info("monorepo artifacts download progress", "current", curr, "total", total)
	}

	monorepoArtifactsLocator := &artifacts.Locator{
		URL: cliConfig.MonorepoArtifactsURL,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	monorepoArtifactsFS, err := artifacts.Download(ctx.Context, monorepoArtifactsLocator, monorepoProgressor, path.Join(homeDir, ".op-deployer/cache"))
	if err != nil {
		return fmt.Errorf("failed to download monorepo artifacts: %w", err)
	}

	// Download periphery contract artifacts
	peripheryProgressor := func(curr, total int64) {
		logger.Info("monorepo artifacts download progress", "current", curr, "total", total)
	}

	peripheryArtifactsLocator := &artifacts.Locator{
		URL: cliConfig.PeripheryArtifactsURL,
	}

	peripheryArtifactsFS, err := artifacts.Download(ctx.Context, peripheryArtifactsLocator, peripheryProgressor, path.Join(homeDir, ".op-deployer/cache"))
	if err != nil {
		return fmt.Errorf("failed to download periphery artifacts: %w", err)
	}

	worldDeployment, worldOutput, err := worldgen.GenerateWorld(ctx.Context, logger, &foundry.ArtifactsFS{FS: monorepoArtifactsFS}, &foundry.ArtifactsFS{FS: peripheryArtifactsFS})

	if err != nil {
		return fmt.Errorf("failed to generate world: %w", err)
	}

	logger.Info("Successfully generated world")

	logger.Info("writing L1 genesis")
	outfile := fmt.Sprintf("%s-l1-genesis.json", worldOutput.L1.Genesis.Config.ChainID)
	if err := jsonutil.WriteJSON(worldOutput.L1.Genesis, ioutil.ToStdOutOrFileOrNoop(path.Join(cliConfig.Outdir, outfile), 0o666)); err != nil {
		return fmt.Errorf("failed to write L1 genesis: %w", err)
	}

	for l2ChainID, l2Deployment := range worldDeployment.L2s {
		logger.Info("writing addresses for l2", "chain_id", l2ChainID)
		outfile := fmt.Sprintf("%s-l2-addresses.json", l2ChainID)
		if err := jsonutil.WriteJSON(l2Deployment, ioutil.ToStdOutOrFileOrNoop(path.Join(cliConfig.Outdir, outfile), 0o666)); err != nil {
			return fmt.Errorf("failed to write addresses: %w", err)
		}
	}

	for l2ChainID, l2Output := range worldOutput.L2s {
		logger.Info("writing genesis for l2", "chain_id", l2ChainID)
		outfile := fmt.Sprintf("%s-l2-genesis.json", l2ChainID)
		l2Genesis := l2Output.Genesis
		if err := jsonutil.WriteJSON(l2Genesis, ioutil.ToStdOutOrFileOrNoop(path.Join(cliConfig.Outdir, outfile), 0o666)); err != nil {
			return fmt.Errorf("failed to write genesis: %w", err)
		}
	}

	return nil
}
