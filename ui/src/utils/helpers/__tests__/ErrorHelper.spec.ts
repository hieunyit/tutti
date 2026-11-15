import { AxiosError } from 'axios';
import { describe, expect, it } from 'vitest';
import { ErrorHelper } from '../ErrorHelper';
import { ProxyAccountHelper } from '../ProxyAccountHelper';

function backendError(reason: string) {
  const error = new AxiosError('Request failed with status code 400');

  error.response = {
    status: 400,
    data: {
      code: 9,
      message: 'FailedPrecondition',
      details: [
        { '@type': 'type.googleapis.com/google.rpc.RequestInfo' },
        { '@type': 'type.googleapis.com/google.rpc.ErrorInfo', reason },
      ],
    },
  } as AxiosError['response'];

  return error;
}

describe('ErrorHelper.stripCode', () => {
  it('removes the gRPC code name in front of a reason only', () => {
    expect(ErrorHelper.stripCode('FailedPrecondition: user is in use')).toBe(
      'user is in use',
    );
    expect(ErrorHelper.stripCode('AlreadyExists: replication x exists')).toBe(
      'replication x exists',
    );
    expect(ErrorHelper.stripCode('bucket "b": already covered')).toBe(
      'bucket "b": already covered',
    );
    expect(ErrorHelper.stripCode(undefined)).toBeUndefined();
  });

  it('reads the reason without the code from an error response', () => {
    const error = backendError('FailedPrecondition: cannot be deleted');

    expect(ErrorHelper.getReason(error)).toBe('cannot be deleted');
    expect(ProxyAccountHelper.getMessage(error)).toBe('cannot be deleted');
  });
});
