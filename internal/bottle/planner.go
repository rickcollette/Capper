package bottle

import "fmt"

// Plan generates a list of resource actions from a bottle spec and
// the caller-supplied parameter values. It does not execute anything.
func Plan(spec BottleSpec, params map[string]string, deploymentName string) ([]PlanAction, error) {
	errs := ValidateSpec(spec, params)
	if len(errs) > 0 {
		var plan []PlanAction
		for _, e := range errs {
			plan = append(plan, PlanAction{Action: "block", Kind: "validation", Name: spec.Metadata.Name, Detail: e})
		}
		return plan, fmt.Errorf("bottle validation failed (%d error(s))", len(errs))
	}

	ctx := BuildContext(spec, params)
	var plan []PlanAction

	// Networks.
	for _, n := range spec.Spec.Resources.Networks {
		name := RenderTemplate(n.Name, ctx)
		plan = append(plan, PlanAction{
			Action: "create",
			Kind:   "network",
			Name:   deploymentName + "-" + name,
			Detail: fmt.Sprintf("mode=%s subnet=%s", n.Mode, n.Subnet),
		})
	}

	// Volumes.
	for _, v := range spec.Spec.Resources.Volumes {
		name := RenderTemplate(v.Name, ctx)
		plan = append(plan, PlanAction{
			Action: "create",
			Kind:   "volume",
			Name:   deploymentName + "-" + name,
			Detail: fmt.Sprintf("type=%s mode=%s size=%s retain=%v", v.Type, v.Mode, v.Size, v.Retain),
		})
	}

	// Secrets.
	for _, sec := range spec.Spec.Resources.Secrets {
		name := RenderTemplate(sec.Name, ctx)
		detail := "value-from-parameter"
		if sec.Generate != nil {
			detail = fmt.Sprintf("generate type=%s length=%d", sec.Generate.Type, sec.Generate.Length)
		}
		plan = append(plan, PlanAction{
			Action: "create",
			Kind:   "secret",
			Name:   deploymentName + "-" + name,
			Detail: detail,
		})
	}

	// Build image if requested.
	if spec.Spec.Build != nil && spec.Spec.Build.Enabled {
		imgName := RenderTemplate(spec.Spec.Build.OutputImage, ctx)
		plan = append(plan, PlanAction{
			Action: "build",
			Kind:   "image",
			Name:   imgName,
			Detail: fmt.Sprintf("base=%s steps=%d", spec.Spec.BaseCapsule, len(spec.Spec.Build.Steps)),
		})
	}

	// Services (instance groups).
	for _, svc := range spec.Spec.Services {
		name := deploymentName + "-" + svc.Name
		image := RenderTemplate(svc.Image, ctx)
		replicas := RenderTemplate(svc.Replicas, ctx)
		if replicas == "" {
			replicas = "1"
		}
		plan = append(plan, PlanAction{
			Action: "create",
			Kind:   "instance-group",
			Name:   name,
			Detail: fmt.Sprintf("image=%s replicas=%s network=%s", image, replicas, svc.Network),
		})
	}

	// Load balancers.
	for _, lb := range spec.Spec.Resources.LoadBalancers {
		name := RenderTemplate(lb.Name, ctx)
		listenPort := RenderTemplate(lb.ListenPort, ctx)
		targetPort := RenderTemplate(lb.TargetPort, ctx)
		plan = append(plan, PlanAction{
			Action: "create",
			Kind:   "lb",
			Name:   deploymentName + "-" + name,
			Detail: fmt.Sprintf("mode=%s listen=%s target=%s:%s", lb.Mode, listenPort, lb.TargetService, targetPort),
		})
	}

	// DNS records.
	for _, dns := range spec.Spec.Resources.DNS {
		host := RenderTemplate(dns.Host, ctx)
		if host == "" {
			host = dns.Name
		}
		plan = append(plan, PlanAction{
			Action: "create",
			Kind:   "dns",
			Name:   host,
			Detail: fmt.Sprintf("target=%s", dns.Target),
		})
	}

	// Capability warnings.
	caps := spec.Spec.Capabilities
	if caps.RequiresPrivileged {
		plan = append(plan, PlanAction{Action: "warn", Kind: "capability", Name: "privileged",
			Detail: "bottle requires privileged execution"})
	}
	if caps.RequiresHostNetwork {
		plan = append(plan, PlanAction{Action: "warn", Kind: "capability", Name: "host-network",
			Detail: "bottle requires host network access"})
	}
	if caps.RequiresExternalDownloads {
		plan = append(plan, PlanAction{Action: "warn", Kind: "capability", Name: "external-downloads",
			Detail: "bottle downloads files from external URLs during build"})
	}

	return plan, nil
}
