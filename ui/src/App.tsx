import * as React from 'react';
import { Backend, StateRequest, ProxyState } from '../generated/web_pb';
import { ProxyClient, ServiceClientOptions, Proxy } from '../generated/web_pb_service';
import { grpc } from 'grpc-web-client';
import './App.css';

// import logo from './logo.svg';



class Button extends React.Component {
  clicker() {
    // we connect to the Web UI port
    // CORS PROBS...
    const client = new ProxyClient("https://127.0.0.1:2016");
    const req = new StateRequest();

    grpc.unary(Proxy.State, {
      request: req,
      host: "https://localhost:2016",
      onEnd: res => {
        const { status, statusMessage, headers, message, trailers } = res;
        if (status === grpc.Code.OK && message) {
          console.log("props of message", message.toObject())
        } else {
          console.log("not okay", res)
        }
      }
    });
  }
  public render() {
    return (
      <button onClick={this.clicker}>Hello</button>
    )
  }
}

class App extends React.Component {
  public render() {
    return (
      <div className="container">
        <header>- co-chair -</header>
        <nav>
          <div><a href="/login">login!!!!</a></div>
          <Button></Button>
          <div>Thing</div>
          <div>Thing</div>
        </nav>
        <main>
          <h1>Main</h1>
          <p>Vestibulum consectetur sit amet nisi ut consectetur. Praesent efficitur, nibh vitae fringilla scelerisque, est neque faucibus quam, in iaculis purus libero eget mauris. Curabitur et luctus sapien, ac gravida orci. Aliquam erat volutpat. In hac habitasse platea dictumst. Aenean commodo, arcu a commodo efficitur, libero dolor mollis turpis, non posuere orci leo eget enim. Curabitur sit amet elementum orci, pulvinar dignissim urna. Morbi id ex eu ex congue laoreet. Aenean tincidunt dolor justo, semper pretium libero luctus nec. Ut vulputate metus accumsan leo imperdiet tincidunt. Phasellus nec rutrum dolor. Cras imperdiet sollicitudin arcu, id interdum nibh fermentum in.</p>
          <p>Vestibulum consectetur sit amet nisi ut consectetur. Praesent efficitur, nibh vitae fringilla scelerisque, est neque faucibus quam, in iaculis purus libero eget mauris. Curabitur et luctus sapien, ac gravida orci. Aliquam erat volutpat. In hac habitasse platea dictumst. Aenean commodo, arcu a commodo efficitur, libero dolor mollis turpis, non posuere orci leo eget enim. Curabitur sit amet elementum orci, pulvinar dignissim urna. Morbi id ex eu ex congue laoreet. Aenean tincidunt dolor justo, semper pretium libero luctus nec. Ut vulputate metus accumsan leo imperdiet tincidunt. Phasellus nec rutrum dolor. Cras imperdiet sollicitudin arcu, id interdum nibh fermentum in.</p>
          <p>Vestibulum consectetur sit amet nisi ut consectetur. Praesent efficitur, nibh vitae fringilla scelerisque, est neque faucibus quam, in iaculis purus libero eget mauris. Curabitur et luctus sapien, ac gravida orci. Aliquam erat volutpat. In hac habitasse platea dictumst. Aenean commodo, arcu a commodo efficitur, libero dolor mollis turpis, non posuere orci leo eget enim. Curabitur sit amet elementum orci, pulvinar dignissim urna. Morbi id ex eu ex congue laoreet. Aenean tincidunt dolor justo, semper pretium libero luctus nec. Ut vulputate metus accumsan leo imperdiet tincidunt. Phasellus nec rutrum dolor. Cras imperdiet sollicitudin arcu, id interdum nibh fermentum in.</p>
        </main>
        <aside>Related links</aside>
        <footer>Footer</footer>
      </div>
    );
  }
}



export default App;
