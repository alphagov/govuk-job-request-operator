package v1

import (
	"fmt"
	"regexp"
	"strings"
)

var validGDSUsersRoles = []string{
	"-fulladmin",
	"-tempadmin",
	"-platformengineer",
	"-developer",
	"-readonly",
	"-ithctester",
}

var validEntraIdUsernameSuffixes = []string{
	"id.internal.digital.gov.uk",
	"gdsbeta.id.digital.gov.uk",
}

type UserIdentity struct {
	AccountId string
	UserName  string
	RoleName  string
}

func ParseUserIdentityFromARN(arn string) (*UserIdentity, error) {
	arnParts := strings.SplitN(arn, ":", 7)
	if !isAssumedRole(arnParts) {
		return nil, fmt.Errorf("%s does not appear to be a valid assumed-role ARN", arn)
	}

	identityParts := strings.SplitN(arnParts[5], "/", 4)
	if len(identityParts) != 3 {
		return nil, fmt.Errorf("%s does not appear to be a valid assumed-role ARN", arn)
	}

	accountId := arnParts[4]
	roleName := identityParts[1]
	sessionName := identityParts[2]

	userIdentity := parseGdsUsersARN(roleName, sessionName)
	if userIdentity == nil {
		userIdentity = parseEntraIDArn(roleName, sessionName)
	}

	if userIdentity == nil {
		return userIdentity, fmt.Errorf("ARN %s could not be parsed as either a GDS Users ARN or an EntraID user ARN", arn)
	}

	userIdentity.AccountId = accountId

	return userIdentity, nil
}

func isAssumedRole(arnParts []string) bool {
	if len(arnParts) != 6 {
		return false
	}

	validAccountId, err := regexp.Match("^[0-9]{12}", []byte(arnParts[4]))
	if err != nil {
		return false
	}

	return (arnParts[0] == "arn" &&
		arnParts[1] == "aws" &&
		arnParts[2] == "sts" &&
		validAccountId &&
		strings.HasPrefix(arnParts[5], "assumed-role"))
}

func parseGdsUsersARN(roleName, sessionName string) *UserIdentity {
	for _, validRole := range validGDSUsersRoles {
		if strings.HasSuffix(roleName, validRole) {
			return &UserIdentity{
				UserName: strings.TrimSuffix(roleName, validRole),
				RoleName: strings.TrimPrefix(validRole, "-"),
			}
		}
	}

	return nil
}

func parseEntraIDArn(roleName, sessionName string) *UserIdentity {
	friendlyRoleName := parseIdentityCenterRole(roleName)
	username := parseIdentityCenterUsername(sessionName)

	if username == "" || friendlyRoleName == "" {
		return nil
	}

	return &UserIdentity{
		UserName: username,
		RoleName: friendlyRoleName,
	}
}

func parseIdentityCenterRole(fullRoleName string) string {
	roleParts := strings.Split(fullRoleName, "_")
	if len(roleParts) < 3 {
		return ""
	}

	if roleParts[0] != "AWSReservedSSO" {
		return ""
	}

	actualRoleParts := roleParts[1 : len(roleParts)-1]

	return strings.Join(actualRoleParts, "_")
}

func parseIdentityCenterUsername(sessionName string) string {
	sessionParts := strings.Split(sessionName, "-")

	if len(sessionParts) < 2 {
		return ""
	}

	actualUsernameParts := sessionParts[0 : len(sessionParts)-1]
	entraIdUsernameDomainPart := sessionParts[len(sessionParts)-1]

	for _, validSuffix := range validEntraIdUsernameSuffixes {
		if strings.HasSuffix(entraIdUsernameDomainPart, validSuffix) {
			return strings.Join(actualUsernameParts, "-")
		}
	}

	return ""
}
