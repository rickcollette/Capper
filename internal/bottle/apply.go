package bottle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// NetworkCreator creates a network and returns its ID.
type NetworkCreator func(name, project, mode, subnet string) (string, error)

// SecretCreator creates a secret with a plaintext value and returns its ID.
type SecretCreator func(name, project, value string) (string, error)

// InstanceGroupCreator launches N instances of an image on a network.
type InstanceGroupCreator func(ctx context.Context, name, project, image, network string, replicas int) ([]string, error)

// LBCreator creates an LB listening on listenPort and returns its ID.
type LBCreator func(name, project, mode, listenPort string) (string, error)

// DNSCreator creates a DNS record and returns its ID.
type DNSCreator func(name, project, host, target string) (string, error)

// ApplyDeps holds callbacks used by the apply engine.
// Using function types keeps bottle/apply.go free of import cycles with
// capper/internal/store.
type ApplyDeps struct {
	CreateNetwork       NetworkCreator
	CreateSecret        SecretCreator
	CreateInstanceGroup InstanceGroupCreator
	CreateLB            LBCreator
	CreateDNS           DNSCreator
}

// DeploymentStore is the minimal store interface the apply engine needs.
// Implemented by bottle/store.Store; defined here to avoid an import cycle.
type DeploymentStore interface {
	UpdateDeploymentStatus(id string, status DeploymentStatus) error
	UpdateDeploymentOutputs(id string, outputs map[string]string, resources []DeployedResource) error
}

// Apply executes a bottle deployment plan against live Capper state.
// It creates resources in dependency order and records each one so the
// deployment can be cleanly removed later.
func Apply(
	ctx context.Context,
	store DeploymentStore,
	deployment BottleDeployment,
	spec BottleSpec,
	params map[string]string,
	deps ApplyDeps,
) (BottleDeployment, error) {
	tctx := BuildContext(spec, params)
	var resources []DeployedResource
	deplName := deployment.Name
	outputs := make(map[string]string)

	_ = store.UpdateDeploymentStatus(deployment.ID, DeploymentDeploying)

	// 1. Networks.
	for _, n := range spec.Spec.Resources.Networks {
		netName := deplName + "-" + RenderTemplate(n.Name, tctx)
		mode := n.Mode
		if mode == "" {
			mode = "nat"
		}
		subnet := n.Subnet
		if subnet == "" || subnet == "auto" {
			subnet = ""
		}
		id, err := deps.CreateNetwork(netName, deployment.Project, mode, subnet)
		if err != nil {
			_ = store.UpdateDeploymentStatus(deployment.ID, DeploymentFailed)
			return deployment, fmt.Errorf("bottle apply: create network %q: %w", netName, err)
		}
		resources = append(resources, DeployedResource{Kind: "network", Name: netName, ID: id})
	}

	// 2. Secrets.
	for _, sec := range spec.Spec.Resources.Secrets {
		secName := deplName + "-" + RenderTemplate(sec.Name, tctx)
		val := ""
		if sec.ValueFromParameter != "" {
			val = tctx["parameters."+sec.ValueFromParameter]
		} else if sec.Generate != nil {
			val = generateSecret(sec.Generate)
		}
		id, err := deps.CreateSecret(secName, deployment.Project, val)
		if err != nil {
			_ = store.UpdateDeploymentStatus(deployment.ID, DeploymentFailed)
			return deployment, fmt.Errorf("bottle apply: create secret %q: %w", secName, err)
		}
		resources = append(resources, DeployedResource{Kind: "secret", Name: secName, ID: id})
		tctx["secrets."+RenderTemplate(sec.Name, tctx)] = id
	}

	// 3. Services (instance groups).
	for _, svc := range spec.Spec.Services {
		svcName := deplName + "-" + svc.Name
		image := RenderTemplate(svc.Image, tctx)
		network := deplName + "-" + svc.Network
		replicas := 1
		if r := RenderTemplate(svc.Replicas, tctx); r != "" {
			if n, err := strconv.Atoi(r); err == nil {
				replicas = n
			}
		}
		ids, err := deps.CreateInstanceGroup(ctx, svcName, deployment.Project, image, network, replicas)
		if err != nil {
			_ = store.UpdateDeploymentStatus(deployment.ID, DeploymentFailed)
			return deployment, fmt.Errorf("bottle apply: create service %q: %w", svcName, err)
		}
		for _, id := range ids {
			resources = append(resources, DeployedResource{Kind: "instance", Name: svcName, ID: id})
		}
		tctx["services."+svc.Name+".name"] = svcName
	}

	// 4. Load balancers.
	for _, lb := range spec.Spec.Resources.LoadBalancers {
		lbName := deplName + "-" + RenderTemplate(lb.Name, tctx)
		listenPort := RenderTemplate(lb.ListenPort, tctx)
		mode := lb.Mode
		if mode == "" {
			mode = "tcp"
		}
		id, err := deps.CreateLB(lbName, deployment.Project, mode, listenPort)
		if err != nil {
			_ = store.UpdateDeploymentStatus(deployment.ID, DeploymentFailed)
			return deployment, fmt.Errorf("bottle apply: create lb %q: %w", lbName, err)
		}
		resources = append(resources, DeployedResource{Kind: "lb", Name: lbName, ID: id})
		tctx["resources.loadBalancers."+RenderTemplate(lb.Name, tctx)+".listenAddress"] = listenPort
	}

	// 5. DNS records.
	for _, dns := range spec.Spec.Resources.DNS {
		host := RenderTemplate(dns.Host, tctx)
		if host == "" {
			host = dns.Name
		}
		target := RenderTemplate(dns.Target, tctx)
		dnsName := deplName + "-dns-" + dns.Name
		id, err := deps.CreateDNS(dnsName, deployment.Project, host, target)
		if err != nil {
			_ = store.UpdateDeploymentStatus(deployment.ID, DeploymentFailed)
			return deployment, fmt.Errorf("bottle apply: create dns %q: %w", host, err)
		}
		resources = append(resources, DeployedResource{Kind: "dns", Name: host, ID: id})
		tctx["resources.dns."+dns.Name+".fqdn"] = host
	}

	// 6. Render outputs.
	for key, outSpec := range spec.Spec.Outputs {
		outputs[key] = RenderTemplate(outSpec.Value, tctx)
	}

	if err := store.UpdateDeploymentOutputs(deployment.ID, outputs, resources); err != nil {
		return deployment, fmt.Errorf("bottle apply: save outputs: %w", err)
	}
	_ = store.UpdateDeploymentStatus(deployment.ID, DeploymentRunning)

	deployment.Status = DeploymentRunning
	deployment.Outputs = outputs
	deployment.Resources = resources
	return deployment, nil
}

func generateSecret(g *SecretGenSpec) string {
	n := g.Length / 2
	if n < 8 {
		n = 16
	}
	b := make([]byte, n)
	_, _ = rand.Read(b)
	s := hex.EncodeToString(b)
	if strings.ToLower(g.Type) == "password" {
		return s[:g.Length]
	}
	return s
}
