"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const react_1 = __importDefault(require("react"));
const DocSidebar = require('@theme/DocSidebar').default;
const MarkdownDescription_1 = __importDefault(require("@khannanov-nil/nil-open-rpc-docs-react/dist/MarkdownDescription/MarkdownDescription"));
const Layout_1 = __importDefault(require("@theme/Layout"));
const path_1 = require("path");
const defaultDescription = `
Explore the JSON-RPC API Documentation for a complete set of reference materials, detailed api descriptions, usage examples, and a live interactive method execution designed to help you test and understand API functionalities in real-time.
`;
function OpenRPCDocIndex(props) {
    const { versionMetadata } = props;
    let sidebar = [
        {
            type: 'link',
            label: props.propsFile.openrpcDocument.info.title || 'JSON-RPC',
            href: props.propsFile.path,
        },
    ].concat(props.propsFile.openrpcDocument.methods.map((method) => {
        return {
            type: 'link',
            label: method.name,
            href: (0, path_1.join)(props.propsFile.path, method.name.toLowerCase()),
        };
    }));
    if (versionMetadata) {
        sidebar = Object.values(versionMetadata.docsSidebars)[0];
    }
    return (react_1.default.createElement(Layout_1.default, null,
        react_1.default.createElement("div", { style: { display: 'flex', width: '100%', flex: '1 0' }, className: "docusaurus-openrpc" },
            react_1.default.createElement("aside", {
                style: {
                    display: 'block',
                    width: 'var(--doc-sidebar-width)',
                    marginTop: 'calc(-1 * var(--ifm-navbar-height))',
                    borderRight: '1px solid var(--ifm-toc-border-color)',
                    willChange: 'width',
                    transition: 'width var(--ifm-transition-fast) ease',
                    clipPath: 'inset(0)',
                }, className: "theme-doc-sidebar-container"
            },
                react_1.default.createElement("div", {
                    style: {
                        top: 0,
                        position: 'sticky',
                        height: '100%',
                        maxHeight: '100vh',
                    }
                },
                    react_1.default.createElement(DocSidebar, { path: props.route.path, sidebar: sidebar }))),
            react_1.default.createElement("main", { className: "docMainContainer", style: { width: '100%' } },
                react_1.default.createElement("div", { className: "container padding-top--md padding-bottom--lg" },
                    react_1.default.createElement("article", null,
                        react_1.default.createElement("h1", null, "=nil; JSON-RPC API"),
                        react_1.default.createElement(MarkdownDescription_1.default, { uiSchema: {}, source: props.propsFile.openrpcDocument.info?.description || defaultDescription }),
                        react_1.default.createElement("h3", null, ""),
                        react_1.default.createElement("h2", null, "Methods"),
                        react_1.default.createElement("ul", null, props.propsFile.openrpcDocument.methods.map((method) => (react_1.default.createElement("li", { key: method.name },
                            react_1.default.createElement("a", { href: `https://docs.nil.foundation/nil/references/${method.name.toLowerCase()}`, target: "_blank", rel: "noopener noreferrer" }, method.name)))))))))));
}
exports.default = OpenRPCDocIndex;
//# sourceMappingURL=OpenRPCDocIndex.js.map