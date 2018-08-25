// package: web
// file: web.proto

var web_pb = require("./web_pb");
var grpc = require("grpc-web-client").grpc;

var Proxy = (function () {
  function Proxy() {}
  Proxy.serviceName = "web.Proxy";
  return Proxy;
}());

Proxy.State = {
  methodName: "State",
  service: Proxy,
  requestStream: false,
  responseStream: false,
  requestType: web_pb.StateRequest,
  responseType: web_pb.ProxyState
};

Proxy.Put = {
  methodName: "Put",
  service: Proxy,
  requestStream: false,
  responseStream: false,
  requestType: web_pb.Backend,
  responseType: web_pb.OpResult
};

Proxy.Remove = {
  methodName: "Remove",
  service: Proxy,
  requestStream: false,
  responseStream: false,
  requestType: web_pb.Backend,
  responseType: web_pb.OpResult
};

Proxy.PutKVStream = {
  methodName: "PutKVStream",
  service: Proxy,
  requestStream: true,
  responseStream: false,
  requestType: web_pb.KV,
  responseType: web_pb.OpResult
};

Proxy.GetKVStream = {
  methodName: "GetKVStream",
  service: Proxy,
  requestStream: false,
  responseStream: true,
  requestType: web_pb.Key,
  responseType: web_pb.KV
};

exports.Proxy = Proxy;

function ProxyClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ProxyClient.prototype.state = function state(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  grpc.unary(Proxy.State, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          callback(Object.assign(new Error(response.statusMessage), { code: response.status, metadata: response.trailers }), null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
};

ProxyClient.prototype.put = function put(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  grpc.unary(Proxy.Put, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          callback(Object.assign(new Error(response.statusMessage), { code: response.status, metadata: response.trailers }), null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
};

ProxyClient.prototype.remove = function remove(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  grpc.unary(Proxy.Remove, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          callback(Object.assign(new Error(response.statusMessage), { code: response.status, metadata: response.trailers }), null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
};

Proxy.prototype.putKVStream = function putKVStream() {
  throw new Error("Bi-directional streaming is not currently supported");
}

ProxyClient.prototype.getKVStream = function getKVStream(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(Proxy.GetKVStream, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onMessage: function (responseMessage) {
      listeners.data.forEach(function (handler) {
        handler(responseMessage);
      });
    },
    onEnd: function (status, statusMessage, trailers) {
      listeners.end.forEach(function (handler) {
        handler();
      });
      listeners.status.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners = null;
    }
  });
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    cancel: function () {
      listeners = null;
      client.close();
    }
  };
};

exports.ProxyClient = ProxyClient;

