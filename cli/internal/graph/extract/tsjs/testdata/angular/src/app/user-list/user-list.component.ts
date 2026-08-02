import { Component, OnInit } from '@angular/core';
import { User, UserService } from '@core/user.service';

@Component({
  selector: 'app-user-list',
  templateUrl: './user-list.component.html',
})
export class UserListComponent implements OnInit {
  users: User[] = [];

  constructor(private service: UserService) {}

  ngOnInit(): void {
    this.users = this.service.list();
  }
}
