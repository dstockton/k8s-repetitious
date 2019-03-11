class Home extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      allDeployments: {},
      deployments: {}
    };

    this.serverRequest = this.serverRequest.bind(this);
  }

  serverRequest() {
    $.get("http://127.0.0.1:3000/api/aggregate-deployments", res => {
      this.setState({
        deployments: res,
        allDeployments: res
      });
      this.filterList();
      setTimeout(this.serverRequest, 1000);
    });
  }

  componentDidMount() {
    this.serverRequest();
  }

  filterMatch(item, searchTerm, failedOnly) {
    return item.metadata.name.toLowerCase().search(searchTerm.toLowerCase()) !== -1;
  }

  filterList() {
    var cluster = document.getElementById('filter-cluster').value;
    var searchTerm = document.getElementById('filter-search').value.toLowerCase();
    var failedOnly = document.getElementById('filter-failedOnly').checked;

    // Yes, it's a bit shit...
    if (this === undefined) {
      return;
    }

    var deployments = this.state.allDeployments;
    deployments = Object.keys(deployments)
      .filter(key =>
          cluster == '*' // all
          ||
          key == cluster // specific cluster
        )
      .reduce((obj, key) => {
        obj[key] = deployments[key]
        if (obj[key].items !=null){
          obj[key].items = obj[key].items.filter(function(item){
            var showIt = item.metadata.name.toLowerCase().search(searchTerm) !== -1;
            showIt = showIt || item.metadata.namespace.toLowerCase().search(searchTerm) !== -1;
            showIt = showIt || item.spec.template.spec.containers.map(container=> {
              return container.image.toLowerCase().includes(searchTerm);
            }).includes(true);
            showIt = showIt && (item.status.readyReplicas != item.status.replicas || !failedOnly);

            return showIt;
          });
        }
        return obj;
      }, {});

    this.setState({
      deployments: deployments
    });
  }

  render() {
    return (
      <div className="container">
        <h2>Deployments</h2>

        <form>
          <div className="row">
            <div className="container">
              <div className="col-xs-5">
                <select id="filter-cluster" class="form-control" onChange={this.filterList}>
                  <option value="*">&laquo; All Clusters &raquo;</option>
                  {
                    Object.keys(this.state.allDeployments).map(cluster=> {
                      if (this.state.allDeployments[cluster].items != null) {
                        return <option>{cluster}</option>;
                      }
                    })
                  }
                </select>
              </div>
              <div className="col-xs-4">
                <input class="form-control" id="filter-search" placeholder="Search" onChange={this.filterList} />
              </div>
              <div className="col-xs-3">
                <div class="form-check form-check-inline">
                  <input class="form-check-input" type="checkbox" id="filter-failedOnly" value="failedOnly" onChange={this.filterList} />
                  <label class="form-check-label" for="filter-failedOnly">Failed Only</label>
                </div>
              </div>
            </div>
          </div>
        </form>

        <br />

        <div className="row">
          <div className="container">
            {
              Object.keys(this.state.deployments).map(cluster=> {
                if (this.state.deployments[cluster].items != null) {
                  return this.state.deployments[cluster].items.map(deployment=> {
                    return <Deployment cluster={cluster} deployment={deployment} />;
                  });
                }
              })
            }
          </div>
        </div>
      </div>
    );
  }
}

class Deployment extends React.Component {
  checkNested(obj) {
    var args = Array.prototype.slice.call(arguments, 1);

    for (var i = 0; i < args.length; i++) {
      if (!obj || !obj.hasOwnProperty(args[i])) {
        return false;
      }
      obj = obj[args[i]];
    }
    return true;
  }

  getReplicas() {
    var r = 0;
    if ( this.checkNested(this.props.deployment, 'status', 'replicas') ) {
      r = this.props.deployment.status.replicas;
    }
    return r;
  }

  getReadyReplicas() {
    var r = 0;
    if ( this.checkNested(this.props.deployment, 'status', 'readyReplicas') ) {
      r = this.props.deployment.status.readyReplicas;
    }
    return r;
  }

  isGoodState() {
    return this.getReadyReplicas() == this.getReplicas();
  }

  getStateClass() {
    if (this.isGoodState()) {
      return 'state-good'
    } else {
      return 'state-bad'
    }
  }

  getContainers() {
    var c = [];
    if ( this.checkNested(this.props.deployment, 'spec', 'template', 'spec', 'containers') ) {
      c = this.props.deployment.spec.template.spec.containers;
    }
    return c;
  }

  render() {
    if (this.props.deployment==null) {
      return 'null1';
    }
    return (
      <div className="col-xs-12">
        <div className="panel panel-default">
          <div className="panel-heading">
            <b>Cluster:</b> {this.props.cluster}
            &nbsp;&nbsp;&nbsp;
            <b>Deployment:</b> {this.props.deployment.metadata.namespace} / {this.props.deployment.metadata.name}
          </div>
          <div className={'panel-body deployment-hld ' + this.getStateClass()}>
            {this.getContainers().map(function(container, i) {
              return <Container key={i} container={container} />;
            })}
          </div>
          <div className="panel-footer">
            {this.getReplicas()} replica{this.getReplicas() > 1 ? 's' : ''}
            {
              !this.isGoodState() &&
              <span> ({this.getReplicas()-this.getReadyReplicas()} failing replica{(this.getReplicas()-this.getReadyReplicas()) > 1 ? 's' : ''})</span>
            }
          </div>
        </div>
      </div>
    );
  }
}

class Container extends React.Component {
  checkNested(obj) {
    var args = Array.prototype.slice.call(arguments, 1);

    for (var i = 0; i < args.length; i++) {
      if (!obj || !obj.hasOwnProperty(args[i])) {
        return false;
      }
      obj = obj[args[i]];
    }
    return true;
  }

  getPorts() {
    var r = [];
    if ( this.checkNested(this.props.container, 'ports') ) {
      r = this.props.container.ports;
    }
    return r;
  }

  getResource(constraint, type) {
    var r = 0;
    if ( this.checkNested(this.props.container.resources, constraint, type) ) {
      r = this.props.container.resources[constraint][type];
    }
    return r;
  }

  render() {
    return (
      <div className="row">
        <div className="container">
        <div className="col-xs-9">
            {this.props.container.image}
            <ul>
              {this.getPorts().map(function(port, i) {
                    return <Port key={i} port={port} />;
              })}
            </ul>
          </div>
          <div className="col-xs-3">
            <b>CPU:</b> {this.getResource('requests', 'cpu')} / {this.getResource('limits', 'cpu')}<br />
            <b>Mem:</b> {this.getResource('requests', 'memory')} / {this.getResource('limits', 'memory')}
            <ul></ul>
          </div>
        </div>
      </div>
    );
  }
}
class Port extends React.Component {
  render() {
    return (
      <li>
        {this.props.port.name} ({this.props.port.containerPort}/{this.props.port.protocol.toLowerCase()})
      </li>
    );
  }
}

ReactDOM.render(<Home />, document.getElementById("app"));
