var sourcemapsApp = angular.module('sourcemapsApp',['uuid']);

var URL_REGEX = /https?:\/\/([A-Za-z0-9\-.]+)(:[0-9]+)?\/([A-Za-z0-9-._~/?#\[\]@!$&'()*+,;=]+)(?=:[0-9]+)/g;

sourcemapsApp.controller('MainController', ['$scope', '$http', 'uuid4', function ($scope, $http, uuid) {

    function Script (url) {
        var _public = {
            url: url,
            mapUrl: null,
            replacement:  uuid.generate(),
            loading: true,
            loadingMap: false,
        };

        return _public;
    }

    $scope.raw = "Error: hoo\n at n (http://localhost/bugsnag/bog.min.js:1:32)\n at o (http://localhost/bugsnag/bog.min.js:1:62)\n at t (http://localhost/bugsnag/bog.min.js:1:79)\n at i (http://localhost/bugsnag/bog.min.js:1:96)\n at u (http://localhost/bugsnag/bog.min.js:1:113) ";

    $scope.loading = {};

    $scope.$watch('raw', function (newVal, oldVal) {
        if (!newVal) {
            return;
        }

        var loading = {};

        $scope.withReplacements = newVal.replace(URL_REGEX, function (url) {
            if (!$scope.loading[url]) {
                $scope.loading[url] = {
                    url: url,
                    replacement: uuid.generate()
                };
            }
            loading[url] = true;

            return $scope.loading[url].replacement;
        });

        Object.keys($scope.loading).forEach(function (url) {
            if (!loading[url]) {
                delete $scope.loading[url];
            }
        });
    });

    $scope.$watch('[loading, raw]', function () {
        var replaced = $scope.withReplacements;
        Object.keys($scope.loading).forEach(function (url) {
            var script = $scope.loading[url];
            replaced = replaced.replace(new RegExp(script.replacement + ":([0-9]+)(?::([0-9]+))?", "g"), function (m, line, col) {
                if (script.map) {
                    var consumer = new sourceMap.SourceMapConsumer(script.map);

                    var position = consumer.originalPositionFor({
                        line: Number(line || 1),
                        column: Number(col || 1)
                    });

                    if (position && position.source) {
                        return [position.source, position.line, position.column].join(":");
                    }
                }
                return [script.url, line, col].join(":");
            });
        });

        $scope.output = replaced;

    }, true);

}]);

sourcemapsApp.controller('LoadController', ['$scope', '$http', function ($scope, $http) {

    $scope.$watch('script.url', function (newVal) {
        if (!newVal) {
            return;
        }
        $scope.script.loading = true;

        $http.get("/get/", {params: {url: $scope.script.url}}).then(function (response) {

            if (response.headers()['x-sourcemap']) {
                $scope.script.mapUrl = response.headers()['x-sourcemap'];
                return;
            }

            response.data.split("\n").forEach(function (line) {
                var match = line.match(/(?:\/\/|\/\*) *# *sourceMappingURL *= *([^ ]*)/);

                if (match) {
                    $scope.script.mapUrl = URI(match[1]).absoluteTo($scope.script.url).toString();
                }
            });

            if (!$scope.script.mapUrl) {
                $scope.script.error = 'No source map comment or header at ' + $scope.script.url;

            }

        }).catch(function (response) {
            if (response.headers()['x-proxy-error']) {
                $scope.script.error = response.headers()['x-proxy-error'];
            } else if (response.status) {
                $scope.script.error =  'GET ' + $scope.script.url + ': HTTP ' + response.status;
            } else {
                $scope.script.error = 'Failed to load';
            }

        }).finally(function () {
            $scope.script.loading = false;
        });

    });

    $scope.$watch('script.mapUrl', function (newVal) {
        if (!newVal) {
            return;
        }
        $scope.script.loadingMap = true;

        $http.get("/get/", {params: {url: $scope.script.mapUrl}}).then(function (response) {
            $scope.script.map = JSON.stringify(response.data);

        }).finally(function () {
            $scope.script.loadingMap = false;
        });

    });

}]);
