# Storefront

The customer-facing shop: browse the catalog, build a cart, check out.

## Running

Requires Node 20+.

```bash
cp .env.example .env   # points at the staging API by default
npm install
npm run dev            # http://localhost:5173
```

## File map

```
src/
  domain/          entities and use case contracts — plain TS, imports nothing
    models/        Product, Cart, Order
    usecases/      LoadProducts, AddCartItem, PlaceOrder — interfaces only
  data/            use case implementations, written against protocols
    usecases/      RemoteLoadProducts, RemoteAddCartItem
    protocols/     HttpClient, CacheStore — what infra has to satisfy
  infra/           adapters for third-party libs and external storage
    http/          AxiosHttpClient
    cache/         LocalStorageAdapter
  presentation/    pages, components, hooks — talks to domain contracts only
    pages/         Catalog, Cart, Checkout
    components/    Button, Money, Skeleton — presentational, no fetching
    hooks/
  main/            factories, dependency injection, routes, app entry
    factories/     makeRemoteLoadProducts, makeCatalogPage
    routes/
```

---

### Out of example above

Configure these points on project lint code: Cyclomatic Complexity, Nested Ternaries 
and Naming Conventions.

Don't forget that rules and skills are important as part of harness too. MCP's,
SPECs and every type of feedfoward should be used to increment the agent context. 
