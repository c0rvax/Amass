// Copyright 2017 Jeff Foley. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package sources

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/OWASP/Amass/amass/core"
	"github.com/OWASP/Amass/amass/utils"
	evbus "github.com/asaskevich/EventBus"
)

// Entrust is the AmassService that handles access to the Entrust data source.
type Entrust struct {
	core.BaseAmassService

	Bus        evbus.Bus
	Config     *core.AmassConfig
	SourceType string
	filter     *utils.StringFilter
}

// NewEntrust requires the enumeration configuration and event bus as parameters.
// The object returned is initialized, but has not yet been started.
func NewEntrust(bus evbus.Bus, config *core.AmassConfig) *Entrust {
	e := &Entrust{
		Bus:        bus,
		Config:     config,
		SourceType: core.CERT,
		filter:     utils.NewStringFilter(),
	}

	e.BaseAmassService = *core.NewBaseAmassService("Entrust", e)
	return e
}

// OnStart implements the AmassService interface
func (e *Entrust) OnStart() error {
	e.BaseAmassService.OnStart()

	go e.startRootDomains()
	return nil
}

// OnStop implements the AmassService interface
func (e *Entrust) OnStop() error {
	e.BaseAmassService.OnStop()
	return nil
}

func (e *Entrust) startRootDomains() {
	// Look at each domain provided by the config
	for _, domain := range e.Config.Domains() {
		e.executeQuery(domain)
	}
}

func (e *Entrust) executeQuery(domain string) {
	u := e.getURL(domain)
	page, err := utils.RequestWebPage(u, nil, nil, "", "")
	if err != nil {
		e.Config.Log.Printf("%s: %s: %v", e.String(), u, err)
		return
	}
	content := strings.Replace(page, "u003d", " ", -1)

	e.SetActive()
	re := e.Config.DomainRegex(domain)
	for _, sd := range re.FindAllString(content, -1) {
		n := cleanName(sd)

		if e.filter.Duplicate(n) {
			continue
		}
		go func(name string) {
			e.Config.MaxFlow.Acquire(1)
			e.Bus.Publish(core.NEWNAME, &core.AmassRequest{
				Name:   name,
				Domain: domain,
				Tag:    e.SourceType,
				Source: e.String(),
			})
		}(n)
	}

	for _, name := range e.extractReversedSubmatches(page) {
		if match := re.FindString(name); match != "" {
			if e.filter.Duplicate(match) {
				continue
			}
			go func(name string) {
				e.Config.MaxFlow.Acquire(1)
				e.Bus.Publish(core.NEWNAME, &core.AmassRequest{
					Name:   name,
					Domain: domain,
					Tag:    e.SourceType,
					Source: e.String(),
				})
			}(match)
		}
	}
}

func (e *Entrust) getURL(domain string) string {
	u, _ := url.Parse("https://ctsearch.entrust.com/api/v1/certificates")

	u.RawQuery = url.Values{
		"fields":         {"subjectO,issuerDN,subjectDN,signAlg,san,sn,subjectCNReversed,cert"},
		"domain":         {domain},
		"includeExpired": {"true"},
		"exactMatch":     {"false"},
		"limit":          {"5000"},
	}.Encode()
	return u.String()
}

func (e *Entrust) extractReversedSubmatches(content string) []string {
	var rev, results []string

	re := regexp.MustCompile("\"valueReversed\": \"(.*)\"")
	for _, subs := range re.FindAllStringSubmatch(content, -1) {
		rev = append(rev, strings.TrimSpace(subs[1]))
	}

	for _, r := range rev {
		s := e.reverseSubdomain(r)

		results = append(results, utils.RemoveAsteriskLabel(s))
	}
	return results
}

func (e *Entrust) reverseSubdomain(name string) string {
	var result []string

	s := strings.Split(name, "")
	for i := len(s) - 1; i >= 0; i-- {
		result = append(result, s[i])
	}
	return strings.Join(result, "")
}
