import { Component, Input } from '@angular/core';
import { User } from '@core/user.service';

@Component({
  selector: 'app-user-card',
  templateUrl: './user-card.component.html',
})
export class UserCardComponent {
  @Input() user!: User;
}
