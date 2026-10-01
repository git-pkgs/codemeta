package codemeta

const nameTerm = "name"

func (d *Document) Name() string                { return d.Get(nameTerm).Text() }
func (d *Document) Description() string         { return d.Get("description").Text() }
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
func (a Agent) Name() string          { return a.Get(nameTerm).Text() }
func (a Agent) GivenName() string     { return a.Get("givenName").Text() }
func (a Agent) FamilyName() string    { return a.Get("familyName").Text() }
func (a Agent) RoleName() Value       { return a.Get("roleName") }
func (a Agent) Identifier() Value     { return a.Get(keywordID) }
func (a Agent) Kind() AgentKind {
	if a.value.kind == String {
		return AgentText
	}
	if a.value.kind != Object {
		return AgentUnknown
	}
	person := a.Get("givenName").kind != Missing || a.Get("familyName").kind != Missing || a.Get("affiliation").kind != Missing
	organization := a.Get("legalName").kind != Missing || a.Get("foundingDate").kind != Missing
	personType, orgType, roleType := a.types()
	if (person && (organization || orgType)) || (organization && personType) || (personType && orgType) {
		return AgentConflict
	}
	if roleType || a.Get("roleName").kind != Missing {
		return AgentRole
	}
	if person {
		return AgentPerson
	}
	if organization {
		return AgentOrganization
	}
	if a.Get(keywordID).kind != Missing && len(a.value.fields) == 1 {
		return AgentReference
	}
	if personType {
		return AgentPerson
	}
	if orgType || a.Get(nameTerm).kind != Missing {
		return AgentOrganization
	}
	return AgentUnknown
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
	if list := value.Get(keywordList); list.kind != Missing {
		value = list
	}
	if set := value.Get(keywordSet); set.kind != Missing {
		value = set
	}
	values := value.Values()
	agents := make([]Agent, len(values))
	for i, v := range values {
		agents[i] = Agent{value: v, terms: scopedTerms(v, terms), relation: relation}
	}
	return agents
}
