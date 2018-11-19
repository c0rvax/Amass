// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package sources

import (
	"fmt"

	"github.com/OWASP/Amass/amass/core"
	"github.com/OWASP/Amass/amass/utils"
	evbus "github.com/asaskevich/EventBus"
)

// VirusTotal is the AmassService that handles access to the VirusTotal data source.
type VirusTotal struct {
	core.BaseAmassService

	Bus        evbus.Bus
	Config     *core.AmassConfig
	SourceType string
	filter     *utils.StringFilter
}

// NewVirusTotal requires the enumeration configuration and event bus as parameters.
// The object returned is initialized, but has not yet been started.
func NewVirusTotal(bus evbus.Bus, config *core.AmassConfig) *VirusTotal {
	v := &VirusTotal{
		Bus:        bus,
		Config:     config,
		SourceType: core.SCRAPE,
		filter:     utils.NewStringFilter(),
	}

	v.BaseAmassService = *core.NewBaseAmassService("VirusTotal", v)
	return v
}

// OnStart implements the AmassService interface
func (v *VirusTotal) OnStart() error {
	v.BaseAmassService.OnStart()

	go v.startRootDomains()
	return nil
}

// OnStop implements the AmassService interface
func (v *VirusTotal) OnStop() error {
	v.BaseAmassService.OnStop()
	return nil
}

func (v *VirusTotal) startRootDomains() {
	// Look at each domain provided by the config
	for _, domain := range v.Config.Domains() {
		v.executeQuery(domain)
	}
}

func (v *VirusTotal) executeQuery(domain string) {
	re := v.Config.DomainRegex(domain)
	url := v.getURL(domain)
	page, err := utils.RequestWebPage(url, nil, nil, "", "")
	if err != nil {
		v.Config.Log.Printf("%s: %s: %v", v.String(), url, err)
		return
	}

	v.SetActive()
	for _, sd := range re.FindAllString(page, -1) {
		n := cleanName(sd)

		if v.filter.Duplicate(n) {
			continue
		}
		go func(name string) {
			v.Config.MaxFlow.Acquire(1)
			v.Bus.Publish(core.NEWNAME, &core.AmassRequest{
				Name:   name,
				Domain: domain,
				Tag:    v.SourceType,
				Source: v.String(),
			})
		}(n)
	}
}

func (v *VirusTotal) getURL(domain string) string {
	format := "https://www.virustotal.com/en/domain/%s/information/"

	return fmt.Sprintf(format, domain)
}
