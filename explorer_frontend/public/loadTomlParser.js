// This script is used to parse TOML files into JSON objects
// It is used in index.html to read .toml configuration files

var tomlParser = (() => {
  var parseGroup = (context, str) => {
    var result = context.result;
    var group = parseGroupName(str);
    if (group.indexOf(".") !== -1) {
      var groups = parseSubGroups(group);
      addGroups(result, groups);
    } else {
      addGroup(result, group);
    }

    function parseGroupName(str) {
      var start = str.indexOf("["),
        end = str.indexOf("]");
      return str.substring(start + 1, end);
    }

    function parseSubGroups(str) {
      return str.split(".");
    }

    function addGroup(result, group) {
      if (result[group]) {
        throw new Error('"' + group + '" is overriding existing value');
      }

      var current = (result[group] = {});
      context.currentGroup = current;
    }

    function addGroups(result, groups) {
      groups.reduce((prev, current) => {
        if (!result[prev]) {
          addGroup(result, prev);
        }
        addGroup(result[prev], current);
        return current;
      });
    }
  };

  var parseExpression = (context, line) => {
    var pair = parseNameValue(line);
    var value = parseValue(pair.value);
    var currentGroup = context.currentGroup || context.result;

    currentGroup[pair.name] = value;

    function parseNameValue(line) {
      var equal = line.indexOf("=");
      return {
        name: line.substring(0, equal),
        value: line.substring(equal + 1),
      };
    }

    function parseValue(value) {
      if (array(value)) {
        return parseArray(value);
      }

      return parsePrimitive(value);

      function array(value) {
        return value.charAt(0) === "[" && value.charAt(value.length - 1) === "]";
      }
    }

    function parseArray(value) {
      var values = parseArrayValues(value);
      return values.map((v) => parseValue(v));

      function parseArrayValues(value) {
        var parsed = [];
        var array = value.substring(1, value.length - 1);
        var map = commasMap(array);
        map.reduce((prev, next) => {
          parsed.push(array.substring(prev + 1, next));
          return next;
        }, -1);

        return parsed;

        function commasMap(value) {
          var map = [];
          for (var index = 0; index < value.length; index++) {
            var element = value[index];
            if (element === "[") {
              depth++;
            } else if (element === "]") {
              depth--;
            }

            if (element === "," && depth === 0) {
              map.push(index);
            }
          }

          map.push(value.length);

          return map;
        }
      }
    }

    function parsePrimitive(value) {
      if (date(value)) {
        return new Date(value);
      }

      return eval(value);

      function date(value) {
        return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z/.test(value);
      }
    }
  };

  var parseLine = (context, line) => {
    if (group(line)) {
      parseGroup(context, line);
    } else if (expression(line)) {
      parseExpression(context, line);
    } else if (empty(line)) {
      resetContext();
    }

    function group(line) {
      return line.charAt(0) === "[";
    }

    function expression(line) {
      return line.indexOf("=") > 0;
    }

    function empty(line) {
      return line === "";
    }

    function resetContext() {
      delete context.currentGroup;
    }
  };

  var parse = (context, lines) => {
    mergeMultilines(lines).forEach((line) => {
      line = stripComments(replaceWhitespaces(line));
      parseLine(context, line);
    });

    function replaceWhitespaces(line) {
      return line.replace(/\s/g, "");
    }

    function stripComments(line) {
      return line.split("#")[0];
    }

    function mergeMultilines(lines) {
      var merged = [],
        acc = [],
        capture = false,
        merge = false;
      lines.forEach((line) => {
        if (multilineArrayStart(line)) {
          capture = true;
        }

        if (capture && multilineArrayEnd(line)) {
          merge = true;
        }

        if (capture) {
          acc.push(line);
        } else {
          merged.push(line);
        }

        if (merge) {
          capture = false;
          merge = false;
          merged.push(acc.join(""));
          acc = [];
        }
      });

      return merged;

      function multilineArrayStart(line) {
        return line.indexOf("[") !== -1 && line.indexOf("]") === -1;
      }

      function multilineArrayEnd(line) {
        return line.indexOf("]") !== -1;
      }
    }
  };

  var startParser = (str) => {
    var context = {};
    context.result = {};
    var lines = str.toString().split("\n");

    parse(context, lines);

    return context.result;
  };

  String.prototype.replaceAll = function (find, replace) {
    return this.replace(new RegExp(find, "g"), replace);
  };

  return {
    parse: startParser,
  };
})();
