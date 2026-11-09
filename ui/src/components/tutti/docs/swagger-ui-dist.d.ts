declare module 'swagger-ui-dist/swagger-ui-es-bundle.js' {
  const SwaggerUIBundle: {
    (config: Record<string, unknown>): { getSystem: () => unknown };
    presets: { apis: unknown };
  };

  export default SwaggerUIBundle;
}
declare module 'swagger-ui-dist/swagger-ui.css';
