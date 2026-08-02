import { Component } from '@angular/core';

@Component({
  selector: 'app-shell',
  template: `
    <header class="shell">Users</header>
    <app-user-list></app-user-list>
  `,
})
export class ShellComponent {}
