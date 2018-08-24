// package: web
// file: web.proto

import * as jspb from "google-protobuf";

export class Backend extends jspb.Message {
  getDomain(): string;
  setDomain(value: string): void;

  clearIpsList(): void;
  getIpsList(): Array<string>;
  setIpsList(value: Array<string>): void;
  addIps(value: string, index?: number): string;

  getHealthCheck(): string;
  setHealthCheck(value: string): void;

  getHealthStatus(): string;
  setHealthStatus(value: string): void;

  getProtocol(): Backend.Protocol;
  setProtocol(value: Backend.Protocol): void;

  hasInternetCert(): boolean;
  clearInternetCert(): void;
  getInternetCert(): X509Cert | undefined;
  setInternetCert(value?: X509Cert): void;

  hasBackendCert(): boolean;
  clearBackendCert(): void;
  getBackendCert(): X509Cert | undefined;
  setBackendCert(value?: X509Cert): void;

  getMatchHeadersMap(): jspb.Map<string, string>;
  clearMatchHeadersMap(): void;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Backend.AsObject;
  static toObject(includeInstance: boolean, msg: Backend): Backend.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Backend, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Backend;
  static deserializeBinaryFromReader(message: Backend, reader: jspb.BinaryReader): Backend;
}

export namespace Backend {
  export type AsObject = {
    domain: string,
    ipsList: Array<string>,
    healthCheck: string,
    healthStatus: string,
    protocol: Backend.Protocol,
    internetCert?: X509Cert.AsObject,
    backendCert?: X509Cert.AsObject,
    matchHeadersMap: Array<[string, string]>,
  }

  export enum Protocol {
    HTTP1 = 0,
    HTTP2 = 1,
    GRPC = 3,
  }
}

export class X509Cert extends jspb.Message {
  getCert(): Uint8Array | string;
  getCert_asU8(): Uint8Array;
  getCert_asB64(): string;
  setCert(value: Uint8Array | string): void;

  getKey(): Uint8Array | string;
  getKey_asU8(): Uint8Array;
  getKey_asB64(): string;
  setKey(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): X509Cert.AsObject;
  static toObject(includeInstance: boolean, msg: X509Cert): X509Cert.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: X509Cert, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): X509Cert;
  static deserializeBinaryFromReader(message: X509Cert, reader: jspb.BinaryReader): X509Cert;
}

export namespace X509Cert {
  export type AsObject = {
    cert: Uint8Array | string,
    key: Uint8Array | string,
  }
}

export class Key extends jspb.Message {
  getPrefix(): Uint8Array | string;
  getPrefix_asU8(): Uint8Array;
  getPrefix_asB64(): string;
  setPrefix(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Key.AsObject;
  static toObject(includeInstance: boolean, msg: Key): Key.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Key, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Key;
  static deserializeBinaryFromReader(message: Key, reader: jspb.BinaryReader): Key;
}

export namespace Key {
  export type AsObject = {
    prefix: Uint8Array | string,
  }
}

export class KV extends jspb.Message {
  getKey(): Uint8Array | string;
  getKey_asU8(): Uint8Array;
  getKey_asB64(): string;
  setKey(value: Uint8Array | string): void;

  getValue(): Uint8Array | string;
  getValue_asU8(): Uint8Array;
  getValue_asB64(): string;
  setValue(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): KV.AsObject;
  static toObject(includeInstance: boolean, msg: KV): KV.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: KV, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): KV;
  static deserializeBinaryFromReader(message: KV, reader: jspb.BinaryReader): KV;
}

export namespace KV {
  export type AsObject = {
    key: Uint8Array | string,
    value: Uint8Array | string,
  }
}

export class ProxyState extends jspb.Message {
  clearBackendsList(): void;
  getBackendsList(): Array<Backend>;
  setBackendsList(value: Array<Backend>): void;
  addBackends(value?: Backend, index?: number): Backend;

  getStatus(): string;
  setStatus(value: string): void;

  getCode(): number;
  setCode(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ProxyState.AsObject;
  static toObject(includeInstance: boolean, msg: ProxyState): ProxyState.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ProxyState, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ProxyState;
  static deserializeBinaryFromReader(message: ProxyState, reader: jspb.BinaryReader): ProxyState;
}

export namespace ProxyState {
  export type AsObject = {
    backendsList: Array<Backend.AsObject>,
    status: string,
    code: number,
  }
}

export class OpResult extends jspb.Message {
  getCode(): number;
  setCode(value: number): void;

  getStatus(): string;
  setStatus(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): OpResult.AsObject;
  static toObject(includeInstance: boolean, msg: OpResult): OpResult.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: OpResult, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): OpResult;
  static deserializeBinaryFromReader(message: OpResult, reader: jspb.BinaryReader): OpResult;
}

export namespace OpResult {
  export type AsObject = {
    code: number,
    status: string,
  }
}

export class StateRequest extends jspb.Message {
  getDomain(): string;
  setDomain(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StateRequest.AsObject;
  static toObject(includeInstance: boolean, msg: StateRequest): StateRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: StateRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StateRequest;
  static deserializeBinaryFromReader(message: StateRequest, reader: jspb.BinaryReader): StateRequest;
}

export namespace StateRequest {
  export type AsObject = {
    domain: string,
  }
}

