package baremetalhardwareprovisioning

import (
	v1 "github.com/openshift-eng/ci-test-mapping/pkg/api/types/v1"
	"github.com/openshift-eng/ci-test-mapping/pkg/config"
)

type Component struct {
	*config.Component
}

var BareMetalHardwareProvisioningComponent = Component{
	Component: &config.Component{
		Name:                 "Bare Metal Hardware Provisioning",
		Operators:            []string{},
		DefaultJiraComponent: "Bare Metal Hardware Provisioning",
		Variants:             []string{"Platform:metal"},
		Matchers: []config.ComponentMatcher{
			{
				IncludeAll: []string{"bz-baremetal"},
			},
			{
				IncludeAll: []string{"bz-Bare Metal Hardware Provisioning"},
			},
		},
	},
}

func (c *Component) IdentifyTest(test *v1.TestInfo) (*v1.TestOwnership, error) {
	if matcher := c.FindMatch(test); matcher != nil {
		jira := matcher.JiraComponent
		if jira == "" {
			jira = c.DefaultJiraComponent
		}
		return &v1.TestOwnership{
			Name:          test.Name,
			Component:     c.Name,
			JIRAComponent: jira,
			Priority:      matcher.Priority,
			Capabilities:  append(matcher.Capabilities, identifyCapabilities(test)...),
		}, nil
	}

	return nil, nil
}

func (c *Component) IdentifyVariants() ([]string, error) {
	return c.Component.IdentifyVariants()
}

func (c *Component) JiraProject() string {
	return c.Component.JiraProject()
}

func (c *Component) StableID(test *v1.TestInfo) string {
	// Look up the stable name for our test in our renamed tests map.
	if stableName, ok := c.TestRenames[test.Name]; ok {
		return stableName
	}
	return test.Name
}

func (c *Component) JiraComponents() (components []string) {
	components = []string{c.DefaultJiraComponent}
	for _, m := range c.Matchers {
		components = append(components, m.JiraComponent)
	}

	return components
}
