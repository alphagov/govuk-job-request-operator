package v1

import (
	"fmt"
	"regexp"
	"strings"
)

var validGDSUsersRoles = [...]string{
	"-fulladmin",
	"-tempadmin",
	"-platformengineer",
	"-developer",
	"-readonly",
	"-ithctester",
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
	if strings.Contains(sessionName, "@") {
		return &UserIdentity{
			UserName: sessionName,
			RoleName: roleName,
		}
	}

	return nil
}
