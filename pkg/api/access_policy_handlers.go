/*
 * Copyright © 2026 Clyso GmbH
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

package api

import (
	"context"
	"fmt"
	"sort"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

func policyToPb(p accesspolicy.Policy) *pb.AccessPolicy {
	return &pb.AccessPolicy{User: p.User, Permission: p.Permission, Buckets: p.Buckets, Description: p.Description}
}

func (h *resilienceHandlers) ListAccessPolicies(ctx context.Context, _ *emptypb.Empty) (*pb.AccessPolicyList, error) {
	list, err := h.policies.List(ctx)
	if err != nil {
		return nil, err
	}
	res := &pb.AccessPolicyList{Policies: make([]*pb.AccessPolicy, 0, len(list))}
	for _, p := range list {
		res.Policies = append(res.Policies, policyToPb(p))
	}
	return res, nil
}

func (h *resilienceHandlers) SetAccessPolicy(ctx context.Context, req *pb.AccessPolicy) (*emptypb.Empty, error) {
	p := accesspolicy.Policy{
		User: req.GetUser(), Permission: req.GetPermission(), Buckets: req.GetBuckets(), Description: req.GetDescription(),
	}
	if err := h.policies.Set(ctx, p); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *resilienceHandlers) DeleteAccessPolicy(ctx context.Context, req *pb.AccessPolicyUser) (*emptypb.Empty, error) {
	if req.GetUser() == "" {
		return nil, fmt.Errorf("%w: access policy: user is required", dom.ErrInvalidArg)
	}
	if err := h.policies.Delete(ctx, req.GetUser()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *resilienceHandlers) ListProxyAccounts(ctx context.Context, _ *emptypb.Empty) (*pb.ProxyAccountList, error) {
	statuses, err := proxystatus.List(ctx, h.redis)
	if err != nil {
		return nil, err
	}
	policies, err := h.policies.List(ctx)
	if err != nil {
		return nil, err
	}
	type acc struct {
		keys, sources map[string]struct{}
	}
	accounts := map[string]*acc{}
	get := func(user string) *acc {
		a, ok := accounts[user]
		if !ok {
			a = &acc{keys: map[string]struct{}{}, sources: map[string]struct{}{}}
			accounts[user] = a
		}
		return a
	}
	for _, st := range statuses {
		for _, a := range st.Accounts {
			x := get(a.User)
			x.keys[a.AccessKeyID] = struct{}{}
			x.sources[a.Source] = struct{}{}
		}
	}
	byUser := make(map[string]accesspolicy.Policy, len(policies))
	for _, p := range policies {
		byUser[p.User] = p
		get(p.User) // policy of an account not (yet) reported by any live proxy
	}
	users := make([]string, 0, len(accounts))
	for u := range accounts {
		users = append(users, u)
	}
	sort.Strings(users)
	res := &pb.ProxyAccountList{Accounts: make([]*pb.ProxyAccount, 0, len(users))}
	for _, u := range users {
		a := accounts[u]
		pol, has := byUser[u]
		if !has {
			pol = accesspolicy.Default(u)
		}
		res.Accounts = append(res.Accounts, &pb.ProxyAccount{
			User: u, AccessKeyIds: sortedKeys(a.keys), Sources: sortedKeys(a.sources),
			Policy: policyToPb(pol), HasPolicy: has,
		})
	}
	return res, nil
}

func sortedKeys(m map[string]struct{}) []string {
	res := make([]string, 0, len(m))
	for k := range m {
		res = append(res, k)
	}
	sort.Strings(res)
	return res
}
