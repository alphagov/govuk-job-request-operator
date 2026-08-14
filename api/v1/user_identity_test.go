package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("UsernameARNParser", func() {
	const ERROR bool = true
	const NO_ERROR bool = false

	DescribeTable("returns the correct useridentity",
		func(toParse string, expectErr bool, expectedUserIdentity *UserIdentity) {
			userIdentity, actualErr := ParseUserIdentityFromARN(toParse)

			if expectErr {
				Expect(userIdentity).To(BeNil())
				Expect(actualErr).To(HaveOccurred())
			} else {
				Expect(userIdentity).To(Equal(expectedUserIdentity))
				Expect(actualErr).NotTo(HaveOccurred())
			}
		},
		Entry("With a gds-users ARN", "arn:aws:sts::123456789012:assumed-role/joe.blogs-platformengineer/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "joe.blogs",
			RoleName:  "platformengineer",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN including a hyphenated first name", "arn:aws:sts::123456789012:assumed-role/joe-joey-joejoe.blogs-platformengineer/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "joe-joey-joejoe.blogs",
			RoleName:  "platformengineer",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN including a hyphenated second name", "arn:aws:sts::123456789012:assumed-role/joe.bloggington-hargreaves-platformengineer/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "joe.bloggington-hargreaves",
			RoleName:  "platformengineer",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN including a both hyphenated names", "arn:aws:sts::123456789012:assumed-role/joe-joey-joejoe.bloggington-hargreaves-platformengineer/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "joe-joey-joejoe.bloggington-hargreaves",
			RoleName:  "platformengineer",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN including a number in the name", "arn:aws:sts::123456789012:assumed-role/joe.blogs2-platformengineer/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "joe.blogs2",
			RoleName:  "platformengineer",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN for a single name", "arn:aws:sts::123456789012:assumed-role/jimothy-platformengineer/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "jimothy",
			RoleName:  "platformengineer",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN for the fulladmin role ", "arn:aws:sts::123456789012:assumed-role/moo.deng-fulladmin/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "moo.deng",
			RoleName:  "fulladmin",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN for the platformengineer role ", "arn:aws:sts::123456789012:assumed-role/ungovernable.princess-platformengineer/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "ungovernable.princess",
			RoleName:  "platformengineer",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN for the tempadmin role ", "arn:aws:sts::123456789012:assumed-role/squiggle.woofington-hargreaves-tempadmin/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "squiggle.woofington-hargreaves",
			RoleName:  "tempadmin",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN for the ithctester role ", "arn:aws:sts::123456789012:assumed-role/pesto.penguin-ithctester/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "pesto.penguin",
			RoleName:  "ithctester",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN for the developer role ", "arn:aws:sts::123456789012:assumed-role/neil.seal-developer/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "neil.seal",
			RoleName:  "developer",
			AccountId: "123456789012"}),
		Entry("With a gds-users ARN for the readonly role ", "arn:aws:sts::123456789012:assumed-role/oh.coco-readonly/test-platformengineer", NO_ERROR, &UserIdentity{
			UserName:  "oh.coco",
			RoleName:  "readonly",
			AccountId: "123456789012"}),
		Entry("With an EntraID ARN", "arn:aws:sts::123456789012:assumed-role/ReadOnlyRole/joe.blogs@dcms.gov.uk", NO_ERROR, &UserIdentity{
			UserName:  "joe.blogs@dcms.gov.uk",
			RoleName:  "ReadOnlyRole",
			AccountId: "123456789012"}),
		Entry("With an EntraID ARN a DSIT domain", "arn:aws:sts::123456789012:assumed-role/ReadOnlyRole/joe.blogs@dsit.gov.uk", NO_ERROR, &UserIdentity{
			UserName:  "joe.blogs@dsit.gov.uk",
			RoleName:  "ReadOnlyRole",
			AccountId: "123456789012"}),
		Entry("With an EntraID ARN a digital.cabinet-office.gov.uk domain", "arn:aws:sts::123456789012:assumed-role/ReadOnlyRole/joe.blogs@digital.cabinet-office.gov.uk", NO_ERROR, &UserIdentity{
			UserName:  "joe.blogs@digital.cabinet-office.gov.uk",
			RoleName:  "ReadOnlyRole",
			AccountId: "123456789012"}),
		Entry("With an EntraID ARN with more than 2 names", "arn:aws:sts::123456789012:assumed-role/ReadOnlyRole/joe.joey.joejoe@dcms.gov.uk", NO_ERROR, &UserIdentity{
			UserName:  "joe.joey.joejoe@dcms.gov.uk",
			RoleName:  "ReadOnlyRole",
			AccountId: "123456789012"}),
		Entry("With an EntraID ARN with only 1 name", "arn:aws:sts::123456789012:assumed-role/ReadOnlyRole/jimothy@dcms.gov.uk", NO_ERROR, &UserIdentity{
			UserName:  "jimothy@dcms.gov.uk",
			RoleName:  "ReadOnlyRole",
			AccountId: "123456789012"}),
		Entry("With an EntraID ARN with numbers in the name", "arn:aws:sts::123456789012:assumed-role/ReadOnlyRole/joe.blogs22@dcms.gov.uk", NO_ERROR, &UserIdentity{
			UserName:  "joe.blogs22@dcms.gov.uk",
			RoleName:  "ReadOnlyRole",
			AccountId: "123456789012"}),
		Entry("With an EntraID ARN with a different role name", "arn:aws:sts::123456789012:assumed-role/Developer/joe.blogs22@dcms.gov.uk", NO_ERROR, &UserIdentity{
			UserName:  "joe.blogs22@dcms.gov.uk",
			RoleName:  "Developer",
			AccountId: "123456789012"}),
		Entry("With an ARN that is not an assumed-role", "arn:aws:sts::123456789012:user/jimothy@dcms.gov.uk", ERROR, nil),
		Entry("With an ARN that is is not either a gds-users user nor an EntraID user", "arn:aws:sts::123456789012:assumed-role/foo/bar", ERROR, nil),
		Entry("With an ARN that is is not an sts ARN", "arn:aws:ec2::123456789012:assumed-role/joe.blogs-readonly/bar", ERROR, nil),
		Entry("With an ARN that has an invalid account ID", "arn:aws:ec2::123456789:assumed-role/joe.blogs-readonly/bar", ERROR, nil),
		Entry("With a string that is not an ARN at all", "wibble", ERROR, nil),
	)
})
