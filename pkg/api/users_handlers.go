package api

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/auth"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/s3users"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

func (h *resilienceHandlers) users() (*s3users.Manager, error) {
	if h.s3users == nil {
		return nil, fmt.Errorf("%w: S3 user management is not available on this server", dom.ErrFailedPrecondition)
	}
	return h.s3users, nil
}

func s3UserToPb(u s3users.User) *pb.S3User {
	res := &pb.S3User{
		Name: u.Name, Complete: u.Complete,
		AccessKeys: &pb.S3UserAccessKeys{Static: int32(u.KeyCounts.Static), DynamicActive: int32(u.KeyCounts.DynamicActive), DynamicDisabled: int32(u.KeyCounts.DynamicDisabled)},
		Policy:     &pb.S3UserPolicy{HasPolicy: u.Policy.HasPolicy, Permission: u.Policy.Permission, Buckets: u.Policy.Buckets},
	}
	for _, s := range u.Storages {
		res.Storages = append(res.Storages, &pb.S3UserStorage{Storage: s.Storage, Configured: s.Configured, Source: s.Source, AccessKeyId: s.AccessKeyID, Mode: string(s.Mode)})
	}
	return res
}

func credFromPb(c *pb.S3UserCredential) s3users.Credential {
	return s3users.Credential{Storage: c.GetStorage(), AccessKeyID: c.GetAccessKeyId(), SecretAccessKey: c.GetSecretAccessKey()}
}

func verifyToPb(r s3users.VerifyResult) *pb.S3UserVerifyResult {
	return &pb.S3UserVerifyResult{Storage: r.Storage, Ok: r.OK, Error: r.Error, LatencyMs: r.LatencyMS}
}

func (h *resilienceHandlers) ListS3Users(ctx context.Context, _ *emptypb.Empty) (*pb.S3UserList, error) {
	m, err := h.users()
	if err != nil {
		return nil, err
	}
	l, err := m.List(ctx)
	if err != nil {
		return nil, err
	}
	res := &pb.S3UserList{Users: make([]*pb.S3User, 0, len(l.Users)), MainStorage: l.MainStorage, Storages: l.Storages, DynamicCredentialsEnabled: l.Dynamic}
	for _, u := range l.Users {
		res.Users = append(res.Users, s3UserToPb(u))
	}
	return res, nil
}

func (h *resilienceHandlers) CreateS3User(ctx context.Context, req *pb.CreateS3UserRequest) (*pb.CreateS3UserResponse, error) {
	m, err := h.users()
	if err != nil {
		return nil, err
	}
	r := s3users.CreateRequest{Name: req.GetName(), Verify: req.GetVerify(), CreateAccessKey: req.GetCreateAccessKey(), Description: req.GetDescription()}
	for _, c := range req.GetCredentials() {
		r.Credentials = append(r.Credentials, credFromPb(c))
	}
	if p := req.Policy; p != nil {
		r.Policy = &accesspolicy.Policy{Permission: p.Permission, Buckets: p.Buckets, Description: p.Description}
	}
	res, err := m.Create(ctx, r)
	if err != nil {
		return nil, err
	}
	out := &pb.CreateS3UserResponse{User: s3UserToPb(res.User)}
	for _, v := range res.Verification {
		out.Verification = append(out.Verification, verifyToPb(v))
	}
	if res.AccessKey != nil {
		out.AccessKey = &pb.ProxyAccountSecret{AccessKeyId: res.AccessKey.AccessKeyID, SecretAccessKey: res.AccessSecret}
	}
	return out, nil
}

func (h *resilienceHandlers) UpdateS3UserCredentials(ctx context.Context, req *pb.UpdateS3UserCredentialsRequest) (*emptypb.Empty, error) {
	m, err := h.users()
	if err != nil {
		return nil, err
	}
	if req.GetUser() == "" || req.GetCredential() == nil {
		return nil, fmt.Errorf("%w: user and credential are required", dom.ErrInvalidArg)
	}
	if err := m.UpdateCredentials(ctx, req.User, credFromPb(req.Credential), req.Verify); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *resilienceHandlers) DeleteS3UserCredentials(ctx context.Context, req *pb.DeleteS3UserCredentialsRequest) (*emptypb.Empty, error) {
	m, err := h.users()
	if err != nil {
		return nil, err
	}
	if req.GetName() == "" || req.GetStorage() == "" {
		return nil, fmt.Errorf("%w: name and storage are required", dom.ErrInvalidArg)
	}
	if err := m.DeleteCredentials(ctx, req.GetName(), req.GetStorage()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *resilienceHandlers) DeleteS3User(ctx context.Context, req *pb.DeleteS3UserRequest) (*pb.DeleteS3UserResponse, error) {
	m, err := h.users()
	if err != nil {
		return nil, err
	}
	res, err := m.Delete(ctx, req.GetName(), req.GetForce())
	if err != nil {
		return nil, err
	}
	return &pb.DeleteS3UserResponse{DeletedStorages: res.DeletedStorages, DeletedAccessKeys: int32(res.DeletedAccessKeys), DeletedPolicy: res.DeletedPolicy}, nil
}

func (h *resilienceHandlers) TestS3UserCredentials(ctx context.Context, req *pb.TestS3UserCredentialsRequest) (*pb.TestS3UserCredentialsResponse, error) {
	m, err := h.users()
	if err != nil {
		return nil, err
	}
	var cred *s3users.Credential
	if req.Credential != nil {
		c := credFromPb(req.Credential)
		cred = &c
	}
	r, err := m.Test(ctx, req.GetUser(), req.GetStorage(), cred)
	if err != nil {
		return nil, err
	}
	return &pb.TestS3UserCredentialsResponse{Ok: r.OK, Error: r.Error, LatencyMs: r.LatencyMS}, nil
}

// consoleUsers returns the auth service or a FailedPrecondition error.
func (h *resilienceHandlers) consoleUsers() (*auth.Service, error) {
	if h.auth == nil || !h.auth.Enabled() {
		return nil, fmt.Errorf("%w: sign-in is disabled (api.auth.enabled=false), console users cannot be managed", dom.ErrFailedPrecondition)
	}
	return h.auth, nil
}

func actorName(ctx context.Context) string {
	if s, ok := auth.FromContext(ctx); ok {
		return s.Username
	}
	return ""
}

func ts(t interface{ IsZero() bool }, v *timestamppb.Timestamp) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return v
}

func consoleUserToPb(u auth.UserInfo) *pb.ConsoleUser {
	return &pb.ConsoleUser{
		Username: u.Username, Role: u.Role, Source: u.Source, Disabled: u.Disabled, MustChangePassword: u.MustChangePassword,
		CreatedAt: ts(u.CreatedAt, timestamppb.New(u.CreatedAt)), UpdatedAt: ts(u.UpdatedAt, timestamppb.New(u.UpdatedAt)),
		LastLoginAt: ts(u.LastLoginAt, timestamppb.New(u.LastLoginAt)), PasswordChangedAt: ts(u.PasswordChangedAt, timestamppb.New(u.PasswordChangedAt)),
	}
}

func (h *resilienceHandlers) ListConsoleUsers(ctx context.Context, _ *emptypb.Empty) (*pb.ConsoleUserList, error) {
	svc, err := h.consoleUsers()
	if err != nil {
		return nil, err
	}
	list, err := svc.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	res := &pb.ConsoleUserList{Users: make([]*pb.ConsoleUser, 0, len(list))}
	for _, u := range list {
		res.Users = append(res.Users, consoleUserToPb(u))
	}
	return res, nil
}

func (h *resilienceHandlers) CreateConsoleUser(ctx context.Context, req *pb.CreateConsoleUserRequest) (*pb.CreateConsoleUserResponse, error) {
	svc, err := h.consoleUsers()
	if err != nil {
		return nil, err
	}
	u, generated, err := svc.CreateUser(ctx, req.GetUsername(), req.GetRole(), req.GetPassword(), req.GetGeneratePassword())
	if err != nil {
		return nil, err
	}
	return &pb.CreateConsoleUserResponse{User: consoleUserToPb(u), GeneratedPassword: generated}, nil
}

func (h *resilienceHandlers) UpdateConsoleUser(ctx context.Context, req *pb.UpdateConsoleUserRequest) (*emptypb.Empty, error) {
	svc, err := h.consoleUsers()
	if err != nil {
		return nil, err
	}
	if err := svc.UpdateUser(ctx, actorName(ctx), req.GetUsername(), req.GetRole(), req.Disabled); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *resilienceHandlers) ResetConsoleUserPassword(ctx context.Context, req *pb.ResetConsoleUserPasswordRequest) (*pb.ResetConsoleUserPasswordResponse, error) {
	svc, err := h.consoleUsers()
	if err != nil {
		return nil, err
	}
	generated, err := svc.ResetPassword(ctx, req.GetUsername(), req.GetPassword(), req.GetGeneratePassword())
	if err != nil {
		return nil, err
	}
	return &pb.ResetConsoleUserPasswordResponse{GeneratedPassword: generated}, nil
}

func (h *resilienceHandlers) DeleteConsoleUser(ctx context.Context, req *pb.ConsoleUserName) (*emptypb.Empty, error) {
	svc, err := h.consoleUsers()
	if err != nil {
		return nil, err
	}
	if err := svc.DeleteUser(ctx, actorName(ctx), req.GetUsername()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
