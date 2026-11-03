# Boilerplate — Service (NestJS)

```ts
import { Injectable } from '@nestjs/common';
import { {Contexto}Repository } from './{contexto}.repository';

export class {Entidade}NotFoundError extends Error {
  constructor(id: number) {
    super(`{entidade} ${id} não encontrado`);
  }
}

@Injectable()
export class {Contexto}Service {
  constructor(private readonly repository: {Contexto}Repository) {}

  // gofi:implements REQ-NN
  async get(id: number) {
    const found = await this.repository.findById(id);
    if (!found) throw new {Entidade}NotFoundError(id);
    return found;
  }
}
```
