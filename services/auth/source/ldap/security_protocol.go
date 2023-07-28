// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

// SecurityProtocol protocol type
type SecurityProtocol int

// Note: new type must be added at the end of list to maintain compatibility.
const (
	SecurityProtocolUnencrypted SecurityProtocol = iota
	SecurityProtocolLDAPS
	SecurityProtocolStartTLS
)

// String returns the name of the SecurityProtocol
func (s SecurityProtocol) String() string {
	return SecurityProtocolNames[s]
}

// Int returns the int value of the SecurityProtocol
func (s SecurityProtocol) Int() int {
	return int(s)
}

// SecurityProtocolNames contains the name of SecurityProtocol values.
var SecurityProtocolNames = map[SecurityProtocol]string{
	SecurityProtocolUnencrypted: "Unencrypted",
	SecurityProtocolLDAPS:       "LDAPS",
	SecurityProtocolStartTLS:    "StartTLS",
}
