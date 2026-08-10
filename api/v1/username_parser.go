package v1

import (
	"fmt"
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

func ParseUsernameFromARN(arn string) (string, error) {
	arnParts := strings.SplitN(arn, "/", 4)
	if len(arnParts) != 3 {
		return "", fmt.Errorf("%s does not appear to be a valid assumed-role ARN", arn)
	}

	arnPrefix := arnParts[0]
	roleName := arnParts[1]
	sessionName := arnParts[2]

	if !isAssumedRole(arnPrefix) {
		return "", fmt.Errorf("%s does not appear to be a valid assumed-role ARN", arn)
	}

	username, ok := parseGdsUsersARN(roleName)
	if ok {
		return username, nil
	}

	username, ok = parseEntraIDArn(sessionName)
	if ok {
		return username, nil
	}

	return "", fmt.Errorf("ARN %s could not be parsed as either a GDS Users ARN or an EntraID user ARN", arn)
}

func isAssumedRole(arnPrefix string) bool {
	arnPrefixParts := strings.SplitN(arnPrefix, ":", 7)

	if len(arnPrefixParts) != 6 {
		return false
	}

	return arnPrefixParts[5] == "assumed-role"
}

func parseGdsUsersARN(role string) (string, bool) {
	for _, validRole := range validGDSUsersRoles {
		if strings.HasSuffix(role, validRole) {
			return strings.TrimSuffix(role, validRole), true
		}
	}

	return "", false
}

func parseEntraIDArn(sessionName string) (string, bool) {
	if strings.Contains(sessionName, "@") {
		return sessionName, true
	}

	return "", false
}
