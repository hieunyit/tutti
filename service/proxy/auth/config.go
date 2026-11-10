/*
 * Copyright © 2023 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package auth

import (
	"fmt"

	"github.com/hieunyit/tutti/pkg/s3"
)

type Config struct {
	AllowV2Signature bool                        `yaml:"allowV2Signature"`
	UseStorage       string                      `yaml:"useStorage"`
	Custom           map[string]s3.CredentialsV4 `yaml:"custom"`
	// RequireSignedContentType rejects SigV4 requests that carry a
	// content-type header not listed in SignedHeaders. Default false: AWS SigV4
	// only requires host and x-amz-* headers to be signed, and clients such as
	// MinIO mc/minio-go do not sign content-type. x-amz-* headers are always
	// required to be signed regardless of this setting (CVE-2026-54330).
	RequireSignedContentType bool `yaml:"requireSignedContentType"`
	// DefaultPolicy decides what a user WITHOUT an access policy may do: "" or "allow-legacy" (default): full
	// access if the user's storage credentials are its own, denied if the user is on a shared connection account;
	// "deny": every user without a policy is denied.
	DefaultPolicy string `yaml:"defaultPolicy"`
}

// ValidateDefaultPolicy checks auth.defaultPolicy.
func (c *Config) ValidateDefaultPolicy() error {
	switch c.DefaultPolicy {
	case "", "allow-legacy", "deny":
		return nil
	}
	return fmt.Errorf("proxy config: auth defaultPolicy must be %q or %q, got %q", "deny", "allow-legacy", c.DefaultPolicy)
}
