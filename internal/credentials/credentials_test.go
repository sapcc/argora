// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCredentials(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credentials Suite")
}

var _ = Describe("Credentials", func() {
	var credentials *Credentials

	BeforeEach(func() {
		credentials = NewDefaultCredentials(&Reader{})

		credentials.BMCUser = "user"
		credentials.BMCPassword = "password"
		credentials.NetboxToken = "token"
	})

	Describe("Validate", func() {
		Context("should succeed when all fields are valid", func() {
			It("should not return an error", func() {
				// when
				err := credentials.Validate()

				// then
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("should return an error when BMCUser is empty", func() {
			It("should return an error", func() {
				// given
				credentials.BMCUser = ""

				// when
				err := credentials.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("bmc user is required"))
			})
		})

		Context("should return an error when BMCPassword is empty", func() {
			It("should return an error", func() {
				// given
				credentials.BMCPassword = ""

				// when
				err := credentials.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("bmc password is required"))
			})
		})

		Context("should return an error when NetboxToken is empty", func() {
			It("should return an error", func() {
				// given
				credentials.NetboxToken = ""

				// when
				err := credentials.Validate()

				// then
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("netbox token is required"))
			})
		})
	})
})

type FileReaderMock struct {
	fileContent map[string]string
	returnError bool
}

func (f *FileReaderMock) ReadFile(fileName string) ([]byte, error) {
	if f.returnError {
		return nil, errors.New("error")
	}
	return []byte(f.fileContent[fileName]), nil
}

var _ = Describe("Reload", func() {
	var credentials *Credentials
	var fileReaderMock *FileReaderMock

	BeforeEach(func() {
		fileReaderMock = &FileReaderMock{
			fileContent: make(map[string]string),
			returnError: false,
		}
		credentials = NewDefaultCredentials(fileReaderMock)
	})

	It("should not return an error when all fields are valid", func() {
		// given
		credentialsJson := `{
			"bmcUser": "user",
			"bmcPassword": "password",
			"netboxToken": "token"
		}`

		fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson

		// when
		err := credentials.Reload()

		// then
		Expect(err).ToNot(HaveOccurred())
		Expect(credentials.BMCUser).To(Equal("user"))
		Expect(credentials.BMCPassword).To(Equal("password"))
		Expect(credentials.NetboxToken).To(Equal("token"))
	})

	It("should return an error when credentials file is missing", func() {
		// given
		fileReaderMock.fileContent["/etc/credentials/credentials.json"] = ""

		// when
		err := credentials.Reload()

		// then
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("unable to read credentials.json: unexpected end of JSON input"))
	})

	It("should return an error when credentials file contains invalid JSON", func() {
		// given
		credentialsJson := `b`

		fileReaderMock.fileContent["/etc/credentials/credentials.json"] = credentialsJson
		// when
		err := credentials.Reload()

		// then
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("unable to read credentials.json: invalid character 'b' looking for beginning of value"))
	})

	It("should return an error when reading file fails", func() {
		// given
		fileReaderMock.returnError = true

		// when
		err := credentials.Reload()

		// then
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("readJSONAndUnmarshal", func() {
	var credentials *Credentials
	var fileReaderMock *FileReaderMock

	BeforeEach(func() {
		fileReaderMock = &FileReaderMock{
			fileContent: make(map[string]string),
			returnError: false,
		}
		credentials = NewDefaultCredentials(fileReaderMock)
	})

	It("should unmarshal the JSON content when the file is read successfully", func() {
		// given
		credentialsJsonContent := `{
				"bmcUser": "user",
				"bmcPassword": "password",
				"netboxToken": "token"
			}`
		fileReaderMock.fileContent["/etc/secret/credentials.json"] = credentialsJsonContent

		// when
		err := credentials.readJSONAndUnmarshal("/etc/secret/credentials.json")

		// then
		Expect(err).ToNot(HaveOccurred())
		Expect(credentials.BMCUser).To(Equal("user"))
		Expect(credentials.BMCPassword).To(Equal("password"))
		Expect(credentials.NetboxToken).To(Equal("token"))
	})

	It("should return an error when the file cannot be read", func() {
		// given
		fileReaderMock.returnError = true

		// when
		err := credentials.readJSONAndUnmarshal("/etc/secret/credentials.json")

		// then
		Expect(err).To(HaveOccurred())
	})

	It("should return an error when the file content is invalid JSON", func() {
		// given
		invalidJsonContent := `invalid json`
		fileReaderMock.fileContent["/etc/secret/credentials.json"] = invalidJsonContent

		// when
		err := credentials.readJSONAndUnmarshal("/etc/secret/credentials.json")

		// then
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("invalid character 'i' looking for beginning of value"))
	})
})
