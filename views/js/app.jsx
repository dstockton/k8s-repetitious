class Home extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      allDeployments: {},
      deployments: {},
      allPods: {},
      pods: {}
    };

    this.serverRequest = this.serverRequest.bind(this);
  }

  serverRequest() {
    $.get("/api/aggregated-resources", res => {
      this.setState({
        deployments: res['deployments'],
        allDeployments: res['deployments'],
        pods: res['pods'],
        allPods: res['pods']
      });
      this.filterList(true);
      setTimeout(this.serverRequest, 1000);
    });
  }

  readCookie(name, fallback) {
    var r = Cookies.get(name);
    if (r == null) {
      return fallback;
    }
    return r;
  }

  readCookies() {
    var search = this.readCookie('filter-search');
    var failedOnly = this.readCookie('filter-failedOnly');

    $('#filter-search').val( search );
    $('#filter-failedOnly').prop('checked', (failedOnly=='true'));
  }

  componentDidMount() {
    this.serverRequest();
    this.readCookies();
  }
/*
  filterMatch(item, searchTerm, failedOnly) {
    return item.metadata.name.toLowerCase().search(searchTerm.toLowerCase()) !== -1;
  }
*/
  filterList() {
    var cluster = $('#filter-cluster').val();
    var searchTerm = $('#filter-search').val().toLowerCase();
    var failedOnly = $('#filter-failedOnly').prop('checked');

    // Yes, it's a bit shit...
    if (this === undefined) {
      Cookies.set('filter-search', searchTerm);
      Cookies.set('filter-failedOnly', failedOnly);
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

      var pods = this.state.allPods;
      pods = Object.keys(pods)
        .filter(key =>
            cluster == '*' // all
            ||
            key == cluster // specific cluster
          )
        .reduce((obj, key) => {
          obj[key] = pods[key]
          if (obj[key].items !=null){
            obj[key].items = obj[key].items.filter(function(item){
              var showIt = item.metadata.name.toLowerCase().search(searchTerm) !== -1;
              showIt = showIt || item.metadata.namespace.toLowerCase().search(searchTerm) !== -1;
              showIt = showIt || item.spec.containers.map(container=> {
                return container.image.toLowerCase().includes(searchTerm);
              }).includes(true);
              showIt = showIt && (!"running".split(',').includes( Object.keys(item.status.containerStatuses[0].state)[0] ) || !failedOnly);

              return showIt;
            });
          }
          return obj;
        }, {});

    this.setState({
      deployments: deployments,
      pods: pods
    });
  }

  render() {
    return (
      <div className="container">
        <h2>Repetitious Monitoring System</h2>

        <form>
          <div className="row">
            <div className="container">
              <div className="col-xs-5">
                <select id="filter-cluster" class="form-control" onChange={this.filterList}>
                  <option value="*">&laquo; All Clusters &raquo;</option>
                  {
                    Object.keys(this.state.allDeployments).map(cluster=> {
                      if (this.state.allDeployments[cluster].items != null) {
                        return <option value={cluster}>{cluster}</option>;
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
            {
              Object.keys(this.state.pods).map(cluster=> {
                if (this.state.pods[cluster].items != null) {
                  return this.state.pods[cluster].items.map(pod=> {
                    return <Pod cluster={cluster} pod={pod} />;
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

class Pod extends React.Component {
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

  isGoodPodState() {
    return "running".split(',').includes(this.firstContainerState());
  }

  getStateClass() {
    if (this.isGoodPodState()) {
      return 'state-good'
    } else {
      return 'state-bad'
    }
  }

  getContainers() {
    var c = [];
    if ( this.checkNested(this.props.pod, 'spec', 'containers') ) {
      c = this.props.pod.spec.containers;
    }
    return c;
  }

  firstContainerState() {
    return Object.keys(this.props.pod.status.containerStatuses[0].state)[0];
  }

  render() {
    if (this.props.pod==null) {
      return 'null1';
    }
    return (
      <div className="col-xs-12">
        <div className="panel panel-default">
          <div className="panel-heading">
            <b>Cluster:</b> {this.props.cluster}
            &nbsp;&nbsp;&nbsp;
            <b>Pod:</b> {this.props.pod.metadata.namespace} / {this.props.pod.metadata.name}
          </div>
          <div className={'panel-body pod-hld ' + this.getStateClass()}>
            {this.getContainers().map(function(container, i) {
              return <Container key={i} container={container} />;
            })}
          </div>
          <div className="panel-footer">
            <div className="row">
              <div className="col-xs-6">
                {
                this.props.pod.metadata.hasOwnProperty('ownerReferences') &&
                <span><strong>Owned by:</strong> {this.props.pod.metadata.ownerReferences[0].kind} / {this.props.pod.metadata.ownerReferences[0].name}</span>
                || <span>Orphaned pod</span>
                }
              </div>
              <div className="col-xs-6">
                <span><strong>Status:</strong> {this.firstContainerState()}</span>
              </div>
            </div>
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
