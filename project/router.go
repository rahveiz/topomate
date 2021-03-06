package project

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"

	"github.com/rahveiz/topomate/config"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rahveiz/topomate/utils"
)

type AddressFamily struct {
	IPv4  bool
	IPv6  bool
	VPNv4 bool
	VPNv6 bool
}

// BGPNbr represents a neighbor configuration for a given router
type BGPNbr struct {
	RemoteAS     int
	UpdateSource string
	ConnCheck    bool
	NextHopSelf  bool
	IfName       string
	RouteMapsIn  []string
	RouteMapsOut []string
	AF           AddressFamily
	RRClient     bool
	RSClient     bool
	Mask         int
}

type OSPFNet struct {
	Prefix string
	Area   int
}

// Router contains informations needed to configure a router.
// It contains elements relative to the container and to the FRR configuration.
type Router struct {
	ID            int
	Hostname      string
	ContainerName string
	CustomImage   string
	Loopback      []net.IPNet
	Links         []*NetInterface
	Neighbors     map[string]*BGPNbr
	NextInterface int
	IGP           struct {
		ISIS struct {
			Level int
			Area  int
		}
		// OSPF []string
		OSPF []OSPFNet
	}
}

func (r *Router) LoID() string {
	if len(r.Loopback) == 0 {
		return ""
	}
	return r.Loopback[0].IP.String()
}

func (r *Router) LoInfo() (string, int) {
	if len(r.Loopback) == 0 {
		return "", 0
	}
	m, _ := r.Loopback[0].Mask.Size()
	return r.Loopback[0].IP.String(), m
}

func (r *Router) NeighborsAF() (af AddressFamily) {
	for _, nbr := range r.Neighbors {
		if !af.IPv4 && nbr.AF.IPv4 {
			af.IPv4 = true
		}
		if !af.IPv6 && nbr.AF.IPv6 {
			af.IPv6 = true
		}
		// if !af.VPNv4 && nbr.AF.VPNv4 {
		// 	af.VPNv4 = true
		// }
		// if !af.VPNv6 && nbr.AF.VPNv6 {
		// 	af.VPNv6 = true
		// }
	}
	return
}

// StartContainer starts the container for the router. If configPath is set,
// it also copies the configuration file from the configured directory to
// the container
func (r *Router) StartContainer(wg *sync.WaitGroup, configPath string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)

	if wg != nil {
		defer wg.Done()
	}

	// Check if container already exists
	var containerID string
	flt := filters.NewArgs(filters.Arg("name", r.ContainerName))
	li, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: flt,
	})
	if err != nil {
		utils.Fatalln(err)
	}
	if len(li) == 0 { // container does not exist yet
		hostCfg := &container.HostConfig{
			CapAdd: []string{"SYS_ADMIN", "NET_ADMIN"},
		}
		// if configPath != "" {
		// 	hostCfg.Mounts = []mount.Mount{
		// 		{
		// 			Type:   mount.TypeBind,
		// 			Source: configPath,
		// 			Target: "/etc/frr/frr.conf",
		// 		},
		// 	}
		// }
		image := config.DockerRouterImage
		if r.CustomImage != "" {
			image = r.CustomImage
		}
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:           image,
			Hostname:        r.Hostname,
			NetworkDisabled: true, // docker networking disabled as we use OVS
		}, hostCfg, nil, nil, r.ContainerName)
		utils.Check(err)
		containerID = resp.ID
	} else { // container exists
		containerID = li[0].ID
	}

	// If configPath is set, copy the configuration into the container
	if configPath != "" {
		r.CopyConfig(configPath)
	}

	// Start container
	if err := cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	if config.VFlag {
		fmt.Println(r.ContainerName, "started.")
	}

}

// StopContainer stops the router container
func (r *Router) StopContainer(wg *sync.WaitGroup, configPath string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	utils.Check(err)

	if wg != nil {
		defer wg.Done()
	}

	if configPath != "" {
		r.SaveConfig(configPath)
	}

	if err := cli.ContainerStop(ctx, r.ContainerName, nil); err != nil {
		panic(err)
	}
}

// CopyConfig copies the configuration file configPath to the configuration
// directory in the container file system.
func (r *Router) CopyConfig(configPath string) {
	_, err := exec.Command(
		"docker",
		"cp",
		configPath,
		r.ContainerName+":/etc/frr/frr.conf",
	).CombinedOutput()
	if err != nil {
		utils.Fatalln(err)
	}
}

func (r *Router) SaveConfig(configPath string) {
	_, err := exec.Command(
		"docker",
		"cp",
		r.ContainerName+":/etc/frr/frr.conf",
		configPath,
	).CombinedOutput()
	if err != nil {
		utils.Fatalln(err)
	}
}

func (r *Router) ReloadConfig() {
	out, err := exec.Command(
		"docker",
		"exec",
		r.ContainerName,
		"vtysh",
		"-b",
	).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s %v\n", r.ContainerName, string(out), err)
	}
}

// StartFRR launches the init script inside the container
func (r *Router) StartFRR() {
	utils.StartFrr(r.ContainerName)
}
