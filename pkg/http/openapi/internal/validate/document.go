package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func ValidateDocument(doc *spec.Document, version string) []error {
	var errs []error
	errs = append(errs, validateDocumentVersionFields(doc, version)...)
	errs = append(errs, validateDocumentInfo(doc, version)...)
	errs = append(errs, validateDocumentTopLevelRequirements(doc, version)...)
	errs = append(errs, validateDocumentServersAndExternalDocs(doc, version)...)

	securitySchemes, componentParameters := resolveDocumentSecurityMaps(doc)
	errs = append(errs, ValidateSecurityRequirements("security", doc.Security, securitySchemes, version)...)

	operationIDs := map[string]string{}
	errs = append(errs, validateDocumentPathsAndWebhooks(
		doc,
		version,
		operationIDs,
		securitySchemes,
		componentParameters,
	)...)
	errs = append(errs, validateDocumentComponentsTagsRefs(
		doc,
		version,
		operationIDs,
		securitySchemes,
		componentParameters,
	)...)
	return errs
}

func validateDocumentVersionFields(doc *spec.Document, version string) []error {
	var errs []error
	if doc.OpenAPI != "" && doc.OpenAPI != version {
		errs = append(errs, Errorf("openapi must be %s, got %s", version, doc.OpenAPI))
	}
	if doc.Self != "" && version != spec.Version320 {
		errs = append(errs, Errorf("$self requires OpenAPI 3.2.0"))
	}
	if doc.Self != "" && !IsURIReference(doc.Self) {
		errs = append(errs, Errorf("$self must be a URI reference"))
	}
	if doc.JSONSchemaDialect != "" && !IsURIReference(doc.JSONSchemaDialect) {
		errs = append(errs, Errorf("jsonSchemaDialect must be a URI"))
	}
	return errs
}

func validateDocumentInfo(doc *spec.Document, version string) []error {
	var errs []error
	if doc.Info.Title == "" {
		errs = append(errs, Errorf("info.title is required"))
	}
	if doc.Info.Version == "" {
		errs = append(errs, Errorf("info.version is required"))
	}
	errs = append(errs, ValidateInfo(doc.Info, version)...)
	return errs
}

func validateDocumentTopLevelRequirements(doc *spec.Document, version string) []error {
	var errs []error
	if reflect.IsOpenAPI30(version) && doc.Paths == nil {
		errs = append(errs, Errorf("paths is required"))
	}
	if reflect.IsOpenAPI30(version) {
		if doc.JSONSchemaDialect != "" {
			errs = append(errs, Errorf("jsonSchemaDialect requires OpenAPI 3.1.x or 3.2.0"))
		}
		if doc.Webhooks != nil {
			errs = append(errs, Errorf("webhooks requires OpenAPI 3.1.x or 3.2.0"))
		}
	}
	if IsOpenAPI31(version) || IsOpenAPI32(version) {
		if doc.Paths == nil && doc.Webhooks == nil && doc.Components == nil {
			errs = append(errs, Errorf("one of paths, components, or webhooks is required"))
		}
	}
	return errs
}

func validateDocumentServersAndExternalDocs(doc *spec.Document, version string) []error {
	var errs []error
	if len(doc.Servers) == 0 {
		errs = append(errs, Infof("servers array is empty"))
	}
	for i := range doc.Servers {
		errs = append(errs, ValidateServer(fmt.Sprintf("servers[%d]", i), &doc.Servers[i], version)...)
	}
	errs = append(errs, ValidateServerNames(doc.Servers, version)...)
	if doc.ExternalDocs != nil && doc.ExternalDocs.URL == "" {
		errs = append(errs, Errorf("externalDocs.url is required"))
	}
	if doc.ExternalDocs != nil && doc.ExternalDocs.URL != "" && !IsURIReference(doc.ExternalDocs.URL) {
		errs = append(errs, Errorf("externalDocs.url must be a URI"))
	}
	return errs
}

func resolveDocumentSecurityMaps(
	doc *spec.Document,
) (map[string]*spec.SecurityScheme, map[string]*spec.Parameter) {
	securitySchemes := map[string]*spec.SecurityScheme{}
	componentParameters := map[string]*spec.Parameter{}
	if doc.Components != nil {
		securitySchemes = doc.Components.SecuritySchemes
		componentParameters = doc.Components.Parameters
	}
	return securitySchemes, componentParameters
}

func validateDocumentPathsAndWebhooks(
	doc *spec.Document,
	version string,
	operationIDs map[string]string,
	securitySchemes map[string]*spec.SecurityScheme,
	componentParameters map[string]*spec.Parameter,
) []error {
	var errs []error
	normalizedPaths := map[string]string{}
	for path, item := range doc.Paths {
		if normalized := NormalizeTemplatedPath(path); normalized != path {
			if previous, exists := normalizedPaths[normalized]; exists && previous != path {
				errs = append(errs, Errorf("path %q conflicts with equivalent templated path %q", path, previous))
			} else {
				normalizedPaths[normalized] = path
			}
		}
		errs = append(errs, ValidatePathItem(
			path, item, version, operationIDs, securitySchemes, componentParameters,
		)...)
	}
	for name, item := range doc.Webhooks {
		errs = append(errs, ValidatePathItemOperations(
			"webhooks."+name,
			item,
			version,
			operationIDs,
			securitySchemes,
			componentParameters,
		)...)
	}
	return errs
}

func validateDocumentComponentsTagsRefs(
	doc *spec.Document,
	version string,
	operationIDs map[string]string,
	securitySchemes map[string]*spec.SecurityScheme,
	componentParameters map[string]*spec.Parameter,
) []error {
	var errs []error
	if doc.Components != nil {
		errs = append(errs, ValidateComponents(
			doc.Components, version, operationIDs, securitySchemes, componentParameters,
		)...)
	}
	errs = append(errs, ValidateTags(doc.Tags, version)...)
	errs = append(errs, ValidateReferenceTargets(doc)...)
	return errs
}

func ValidateTags(tags []spec.Tag, version string) []error {
	var errs []error
	tagByName := make(map[string]int, len(tags))
	for i, tag := range tags {
		if tag.Name == "" {
			errs = append(errs, Errorf("tags[%d].name is required", i))
		} else if previous, exists := tagByName[tag.Name]; exists {
			errs = append(errs, Errorf("tags[%d].name %q duplicates tags[%d].name", i, tag.Name, previous))
		} else {
			tagByName[tag.Name] = i
		}
		if version != spec.Version320 && (tag.Summary != "" || tag.Parent != "" || tag.Kind != "") {
			errs = append(errs, Errorf("tags[%d] summary, parent, and kind require OpenAPI 3.2.0", i))
		}
		if tag.ExternalDocs != nil && tag.ExternalDocs.URL == "" {
			errs = append(errs, Errorf("tags[%d].externalDocs.url is required", i))
		}
		if tag.ExternalDocs != nil && tag.ExternalDocs.URL != "" && !IsURIReference(tag.ExternalDocs.URL) {
			errs = append(errs, Errorf("tags[%d].externalDocs.url must be a URI", i))
		}
	}
	if version == spec.Version320 {
		errs = append(errs, ValidateTagParents(tags, tagByName)...)
	}
	return errs
}

func ValidateTagParents(tags []spec.Tag, tagByName map[string]int) []error {
	var errs []error
	for i, tag := range tags {
		if tag.Name == "" || tag.Parent == "" {
			continue
		}
		if _, exists := tagByName[tag.Parent]; !exists {
			errs = append(errs, Errorf("tags[%d].parent %q must reference an existing tag", i, tag.Parent))
			continue
		}
		seen := map[string]bool{tag.Name: true}
		for parent := tag.Parent; parent != ""; {
			if seen[parent] {
				errs = append(errs, Errorf("tags[%d].parent creates a circular tag parent reference", i))
				break
			}
			seen[parent] = true
			parentIndex := tagByName[parent]
			parent = tags[parentIndex].Parent
		}
	}
	return errs
}

//nolint:nestif // straightforward validation rules for the info object.
func ValidateInfo(info spec.Info, version string) []error {
	var errs []error
	if reflect.IsOpenAPI30(version) && info.Summary != "" {
		errs = append(errs, Errorf("info.summary requires OpenAPI 3.1.x or 3.2.0"))
	}
	if info.TermsOfService != nil && !IsURIReference(*info.TermsOfService) {
		errs = append(errs, Errorf("info.termsOfService must be a URI"))
	}
	if info.Contact != nil && info.Contact.URL != "" && !IsURIReference(info.Contact.URL) {
		errs = append(errs, Errorf("info.contact.url must be a URI"))
	}
	if info.Contact != nil && info.Contact.Email != "" && !strings.Contains(info.Contact.Email, "@") {
		errs = append(errs, Errorf("info.contact.email must be an email address"))
	}
	if info.Contact == nil || (info.Contact.Name == "" && info.Contact.URL == "" && info.Contact.Email == "") {
		errs = append(errs, Infof("info.contact is recommended"))
	}
	if info.License != nil {
		if info.License.Name == "" {
			errs = append(errs, Errorf("info.license.name is required"))
		}
		if info.License.URL != "" && !IsURIReference(info.License.URL) {
			errs = append(errs, Errorf("info.license.url must be a URI"))
		}
		if reflect.IsOpenAPI30(version) && info.License.Identifier != "" {
			errs = append(errs, Errorf("info.license.identifier requires OpenAPI 3.1.x or 3.2.0"))
		}
		if info.License.Identifier != "" && info.License.URL != "" {
			errs = append(errs, Errorf("info.license.identifier and info.license.url are mutually exclusive"))
		}
	} else {
		errs = append(errs, Infof("info.license is recommended"))
	}
	return errs
}

func ValidateServerNames(servers []spec.Server, version string) []error {
	var errs []error
	serverNames := map[string]int{}
	for i, server := range servers {
		if server.Name != "" {
			if version != spec.Version320 {
				errs = append(errs, Errorf("servers[%d].name requires OpenAPI 3.2.0", i))
			}
			if previous, exists := serverNames[server.Name]; exists {
				errs = append(errs,
					Errorf("servers[%d].name %q duplicates servers[%d].name", i, server.Name, previous))
			} else {
				serverNames[server.Name] = i
			}
		}
	}
	return errs
}

//nolint:gocognit // straightforward validation rules for the server object.
func ValidateServer(context string, server *spec.Server, version string) []error {
	var errs []error
	if server == nil {
		return nil
	}
	if server.URL == "" {
		errs = append(errs, Errorf("%s.url is required", context))
	} else if !IsServerURL(server.URL) {
		errs = append(errs, Errorf("%s.url must not contain a query or fragment", context))
	}
	if version == spec.Version320 && server.URL != "" {
		seen := map[string]int{}
		for _, m := range pathParamRe.FindAllStringSubmatch(server.URL, -1) {
			seen[m[1]]++
		}
		for varName, count := range seen {
			if count > 1 {
				errs = append(errs, Errorf("%s.url variable {%s} must not appear more than once", context, varName))
			}
		}
	}
	for name, variable := range server.Variables {
		if variable.Default == "" {
			errs = append(errs, Errorf("%s.variables.%s.default is required", context, name))
		}
		if variable.Enum != nil && len(variable.Enum) == 0 {
			if IsOpenAPI31(version) || IsOpenAPI32(version) {
				errs = append(errs, Errorf("%s.variables.%s.enum must not be empty", context, name))
			} else {
				errs = append(errs, Warningf("%s.variables.%s.enum should not be empty", context, name))
			}
		}
		if len(variable.Enum) > 0 && variable.Default != "" && !slices.Contains(variable.Enum, variable.Default) {
			if IsOpenAPI31(version) || IsOpenAPI32(version) {
				errs = append(errs, Errorf("%s.variables.%s.default must be one of enum values", context, name))
			} else {
				errs = append(errs, Warningf("%s.variables.%s.default should be one of enum values", context, name))
			}
		}
	}
	return errs
}
