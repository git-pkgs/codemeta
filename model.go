package codemeta

const nameTerm = "name"

func (d *Document) Name() string                { return firstString(d.Strings(nameTerm)) }
func (d *Document) Description() string         { return firstString(d.Strings("description")) }
func (d *Document) CodeRepository() Value       { return d.Get("codeRepository") }
func (d *Document) SoftwareVersion() Value      { return d.Get("version") }
func (d *Document) License() Value              { return d.Get("license") }
func (d *Document) Keywords() Value             { return d.Get("keywords") }
func (d *Document) ProgrammingLanguages() Value { return d.Get("programmingLanguage") }
func (d *Document) DatePublished() Value        { return d.Get("datePublished") }
func (d *Document) DateModified() Value         { return d.Get("dateModified") }
func (d *Document) DevelopmentStatus() Value    { return d.Get("developmentStatus") }
func (d *Document) Identifier() Value           { return d.Get("identifier") }
func (d *Document) Author() []Agent             { return d.agents("author") }
func (d *Document) Contributor() []Agent        { return d.agents("contributor") }
func (d *Document) Maintainer() []Agent         { return d.agents("maintainer") }
func (d *Document) CopyrightHolder() []Agent    { return d.agents("copyrightHolder") }
func (d *Document) Funder() []Agent             { return d.agents("funder") }

// AgentKind describes the retained form, including conflicting person fields.
type AgentKind uint8

const (
	AgentUnknown AgentKind = iota
	AgentText
	AgentReference
	AgentPerson
	AgentOrganization
	AgentRole
	AgentConflict
)

// Agent is an immutable view. Value retains the original object or scalar.
type Agent struct {
	value    Value
	terms    map[string]termDefinition
	relation string
}

func (a Agent) Value() Value          { return a.value }
func (a Agent) Get(term string) Value { return resolvedGet(a.value, term, a.terms) }
func (a Agent) Name() string {
	if a.value.kind == String {
		return a.value.Text()
	}
	return firstString(a.Strings(nameTerm))
}
func (a Agent) GivenName() string  { return firstString(a.Strings("givenName")) }
func (a Agent) FamilyName() string { return firstString(a.Strings("familyName")) }
func (a Agent) RoleName() Value    { return a.Get("roleName") }
func (a Agent) Identifier() Value  { return a.Get(keywordID) }
func (a Agent) Kind() AgentKind {
	if a.value.kind == String {
		return AgentText
	}
	if a.value.kind != Object {
		return AgentUnknown
	}
	person := hasAgentValue(a.Get("givenName")) || hasAgentValue(a.Get("familyName")) || hasAgentValue(a.Get("affiliation"))
	organization := hasAgentValue(a.Get("legalName")) || hasAgentValue(a.Get("foundingDate"))
	personType, orgType, roleType := a.types()
	if (person && (organization || orgType)) || (organization && personType) || (personType && orgType) {
		return AgentConflict
	}
	if roleType || hasAgentValue(a.Get("roleName")) {
		return AgentRole
	}
	if person {
		return AgentPerson
	}
	if organization {
		return AgentOrganization
	}
	if a.referenceOnly() {
		return AgentReference
	}
	if personType {
		return AgentPerson
	}
	if orgType || hasAgentValue(a.Get(nameTerm)) {
		return AgentOrganization
	}
	return AgentUnknown
}

func (a Agent) referenceOnly() bool {
	if a.Identifier().kind != String {
		return false
	}
	for _, field := range a.value.fields {
		name := expandIRI(field.Name, a.terms)
		if name != keywordID && name != keywordContext && hasAgentValue(field.Value) {
			return false
		}
	}
	return true
}

func hasAgentValue(v Value) bool {
	switch v.kind {
	case Missing, Null:
		return false
	case Array:
		for _, item := range v.items {
			if hasAgentValue(item) {
				return true
			}
		}
		return false
	case Object:
		for _, key := range []string{keywordList, keywordSet, keywordValue} {
			if field := v.Get(key); field.kind != Missing {
				return hasAgentValue(field)
			}
		}
	}
	return true
}
func (a Agent) types() (person, organization, role bool) {
	for _, typ := range a.Get(keywordType).Values() {
		switch expandIRI(typ.text, a.terms) {
		case "https://schema.org/Person":
			person = true
		case "https://schema.org/Organization":
			organization = true
		case "https://schema.org/Role":
			role = true
		}
	}
	return
}

// Agents returns the agents nested under this role's original relationship.
func (a Agent) Agents() []Agent { return agentValues(a.Get(a.relation), a.terms, a.relation) }
func (d *Document) agents(term string) []Agent {
	if d == nil {
		return nil
	}
	var terms map[string]termDefinition
	if d.context.usable() {
		terms = contextTerms[d.context.version]
	}
	return agentValues(d.Get(term), terms, term)
}
func agentValues(value Value, terms map[string]termDefinition, relation string) []Agent {
	var agents []Agent
	for _, v := range value.Values() {
		if v.kind == Null {
			continue
		}
		localTerms := scopedTerms(v, terms)
		if list := v.Get(keywordList); list.kind != Missing {
			agents = append(agents, agentValues(list, localTerms, relation)...)
		} else if set := v.Get(keywordSet); set.kind != Missing {
			agents = append(agents, agentValues(set, localTerms, relation)...)
		} else if v.kind == Array {
			agents = append(agents, agentValues(v, localTerms, relation)...)
		} else {
			agents = append(agents, Agent{value: v, terms: localTerms, relation: relation})
		}
	}
	return agents
}
