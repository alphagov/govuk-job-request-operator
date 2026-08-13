package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("UsernameARNParser", func() {
	DescribeTable("returns the correct username",
		func(toParse, expectedUsername string, expectErr bool) {
			actualUsername, actualErr := ParseUsernameFromARN(toParse)
			Expect(actualUsername).To(Equal(expectedUsername))
			if expectErr {
				Expect(actualErr).To(HaveOccurred())
			} else {
				Expect(actualErr).NotTo(HaveOccurred())
			}
		},
		Entry("With a gds-users ARN", "arn:aws:sts::123456789:assumed-role/joe.blogs-platformengineer/test-platformengineer", "joe.blogs", false),
		Entry("With a gds-users ARN including a hyphenated first name", "arn:aws:sts::123456789:assumed-role/joe-joey-joejoe.blogs-platformengineer/test-platformengineer", "joe-joey-joejoe.blogs", false),
		Entry("With a gds-users ARN including a hyphenated second name", "arn:aws:sts::123456789:assumed-role/joe.bloggington-hargreaves-platformengineer/test-platformengineer", "joe.bloggington-hargreaves", false),
		Entry("With a gds-users ARN including a both hyphenated names", "arn:aws:sts::123456789:assumed-role/joe-joey-joejoe.bloggington-hargreaves-platformengineer/test-platformengineer", "joe-joey-joejoe.bloggington-hargreaves", false),
		Entry("With a gds-users ARN including a number in the name", "arn:aws:sts::123456789:assumed-role/joe.blogs2-platformengineer/test-platformengineer", "joe.blogs2", false),
		Entry("With a gds-users ARN for a single name", "arn:aws:sts::123456789:assumed-role/jimothy-platformengineer/test-platformengineer", "jimothy", false),
		Entry("With a gds-users ARN for the fulladmin role ", "arn:aws:sts::123456789:assumed-role/moo.deng-fulladmin/test-platformengineer", "moo.deng", false),
		Entry("With a gds-users ARN for the platformengineer role ", "arn:aws:sts::123456789:assumed-role/ungovernable.princess-platformengineer/test-platformengineer", "ungovernable.princess", false),
		Entry("With a gds-users ARN for the tempadmin role ", "arn:aws:sts::123456789:assumed-role/squiggle.woofington-hargreaves-tempadmin/test-platformengineer", "squiggle.woofington-hargreaves", false),
		Entry("With a gds-users ARN for the ithctester role ", "arn:aws:sts::123456789:assumed-role/pesto.penguin-ithctester/test-platformengineer", "pesto.penguin", false),
		Entry("With a gds-users ARN for the developer role ", "arn:aws:sts::123456789:assumed-role/neil.seal-developer/test-platformengineer", "neil.seal", false),
		Entry("With a gds-users ARN for the readonly role ", "arn:aws:sts::123456789:assumed-role/oh.coco-readonly/test-platformengineer", "oh.coco", false),
		Entry("With an EntraID ARN", "arn:aws:sts::123456789:assumed-role/ReadOnlyRole/joe.blogs@dcms.gov.uk", "joe.blogs@dcms.gov.uk", false),
		Entry("With an EntraID ARN a DSIT domain", "arn:aws:sts::123456789:assumed-role/ReadOnlyRole/joe.blogs@dsit.gov.uk", "joe.blogs@dsit.gov.uk", false),
		Entry("With an EntraID ARN a digital.cabinet-office.gov.uk domain", "arn:aws:sts::123456789:assumed-role/ReadOnlyRole/joe.blogs@digital.cabinet-office.gov.uk", "joe.blogs@digital.cabinet-office.gov.uk", false),
		Entry("With an EntraID ARN with more than 2 names", "arn:aws:sts::123456789:assumed-role/ReadOnlyRole/joe.joey.joejoe@dcms.gov.uk", "joe.joey.joejoe@dcms.gov.uk", false),
		Entry("With an EntraID ARN with only 1 name", "arn:aws:sts::123456789:assumed-role/ReadOnlyRole/jimothy@dcms.gov.uk", "jimothy@dcms.gov.uk", false),
		Entry("With an EntraID ARN with numbers in the name", "arn:aws:sts::123456789:assumed-role/ReadOnlyRole/joe.blogs22@dcms.gov.uk", "joe.blogs22@dcms.gov.uk", false),
		Entry("With an ARN that is not an assumed-role", "arn:aws:sts::123456789:user/jimothy@dcms.gov.uk", "", true),
		Entry("With an ARN that is is not either a gds-users user nor an EntraID user", "arn:aws:sts::123456789:assumed-role/foo/bar", "", true),
		Entry("With a string that is not an ARN at all", "wibble", "", true),
	)
})
