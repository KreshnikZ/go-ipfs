package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	libp2p "github.com/ipfs/go-ipfs/core/node/libp2p"
	icore "github.com/ipfs/interface-go-ipfs-core"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

func setupPlugins(externalPluginsPath string) error {
	// Load any external plugins if available on externalPluginsPath
	plugins, err := loader.NewPluginLoader(filepath.Join(externalPluginsPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	// Load preloaded and external plugins
	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	return nil
}

func createTempRepo(ctx context.Context) (string, error) {
	repoPath, err := ioutil.TempDir("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return "", err
	}

	// Create the repo with the config
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}

//Creates an IPFS node and returns its CoreAPI
func createNode(ctx context.Context, repoPath string) (icore.CoreAPI, error) {
	//Open the repo
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, err
	}

	//Construct the node

	nodeOptions := &core.BuildCfg{
		//DIFFERENT FROM GUIDE: making this Offline
		Online:  false,
		Routing: libp2p.DHTOption, // This option sets the node to be a full DHT node (both fetching and storing DHT Records)
		Repo:    repo,
	}

	node, err := core.NewNode(ctx, nodeOptions)
	if err != nil {
		return nil, err
	}

	//Attach the Core API to the constructed node

	return coreapi.NewCoreAPI(node)

}

//Spawns a node to be used just for this run (i.e. creates a temporary repo)
func spawnEphemeral(ctx context.Context) (icore.CoreAPI, error) {
	if err := setupPlugins(""); err != nil {
		return nil, err
	}

	//Create a temporary repo
	repoPath, err := createTempRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp repo: %s", err)
	}

	//Spawning an ephemeral IPFS node
	return createNode(ctx, repoPath)

}

func getUnixfsNode(path string) (files.Node, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := files.NewSerialFile(path, false, st)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func main() {
	fmt.Println("Getting an IPFS node running")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//Spawn a node using a temporary path, creating a temporary repo for the run

	fmt.Println("Spawning node on a temporary repo")

	ipfs, err := spawnEphemeral(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	fmt.Println("IPFS node is running")

	// Adding a file to IPFS

	fmt.Println("Adding a file")

	inputBasePath := "./files/"
	inputPathFile := inputBasePath + "file.txt"

	//TO DO: try using the getUnixfsFile and not getUnixfsNode

	someFile, err := getUnixfsNode(inputPathFile)
	if err != nil {
		panic(fmt.Errorf("Could not get File: &s", err))
	}

	cidFile, err := ipfs.Unixfs().Add(ctx, someFile)
	if err != nil {
		panic(fmt.Errorf("Could not add file: %s", err))
	}

	fmt.Println("Added file to IPFS with CID %s\n", cidFile.String())

	// Getting the file and storing it in the folder Downloads

	outputBasePath := "./Downloads/"
	outputPathFile := outputBasePath + strings.Split(cidFile.String(), "/")[2]

	rootNodeFile, err := ipfs.Unixfs().Get(ctx, cidFile)
	if err != nil {
		panic(fmt.Errorf("Could not get file with CID: %s", err))
	}

	err = files.WriteTo(rootNodeFile, outputPathFile)
	if err != nil {
		panic(fmt.Errorf("Could not write out the fetched CID: %s", err))
	}

	fmt.Printf("Got file back from IPFS (IPFS path: %s) and wrote it to %s\n", cidFile.String(), outputPathFile)

}