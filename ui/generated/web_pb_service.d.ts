// package: web
// file: web.proto

import * as web_pb from "./web_pb";
import {grpc} from "grpc-web-client";

type ProxyState = {
  readonly methodName: string;
  readonly service: typeof Proxy;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof web_pb.StateRequest;
  readonly responseType: typeof web_pb.ProxyState;
};

type ProxyPut = {
  readonly methodName: string;
  readonly service: typeof Proxy;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof web_pb.Backend;
  readonly responseType: typeof web_pb.OpResult;
};

type ProxyRemove = {
  readonly methodName: string;
  readonly service: typeof Proxy;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof web_pb.Backend;
  readonly responseType: typeof web_pb.OpResult;
};

type ProxyPutKVStream = {
  readonly methodName: string;
  readonly service: typeof Proxy;
  readonly requestStream: true;
  readonly responseStream: false;
  readonly requestType: typeof web_pb.KV;
  readonly responseType: typeof web_pb.OpResult;
};

type ProxyGetKVStream = {
  readonly methodName: string;
  readonly service: typeof Proxy;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof web_pb.Key;
  readonly responseType: typeof web_pb.KV;
};

export class Proxy {
  static readonly serviceName: string;
  static readonly State: ProxyState;
  static readonly Put: ProxyPut;
  static readonly Remove: ProxyRemove;
  static readonly PutKVStream: ProxyPutKVStream;
  static readonly GetKVStream: ProxyGetKVStream;
}

export type ServiceError = { message: string, code: number; metadata: grpc.Metadata }
export type Status = { details: string, code: number; metadata: grpc.Metadata }
export type ServiceClientOptions = { transport: grpc.TransportConstructor; debug?: boolean }

interface ResponseStream<T> {
  cancel(): void;
  on(type: 'data', handler: (message: T) => void): ResponseStream<T>;
  on(type: 'end', handler: () => void): ResponseStream<T>;
  on(type: 'status', handler: (status: Status) => void): ResponseStream<T>;
}

export class ProxyClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: ServiceClientOptions);
  state(
    requestMessage: web_pb.StateRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError, responseMessage: web_pb.ProxyState|null) => void
  ): void;
  state(
    requestMessage: web_pb.StateRequest,
    callback: (error: ServiceError, responseMessage: web_pb.ProxyState|null) => void
  ): void;
  put(
    requestMessage: web_pb.Backend,
    metadata: grpc.Metadata,
    callback: (error: ServiceError, responseMessage: web_pb.OpResult|null) => void
  ): void;
  put(
    requestMessage: web_pb.Backend,
    callback: (error: ServiceError, responseMessage: web_pb.OpResult|null) => void
  ): void;
  remove(
    requestMessage: web_pb.Backend,
    metadata: grpc.Metadata,
    callback: (error: ServiceError, responseMessage: web_pb.OpResult|null) => void
  ): void;
  remove(
    requestMessage: web_pb.Backend,
    callback: (error: ServiceError, responseMessage: web_pb.OpResult|null) => void
  ): void;
  putKVStream(): void;
  getKVStream(requestMessage: web_pb.Key, metadata?: grpc.Metadata): ResponseStream<web_pb.KV>;
}

