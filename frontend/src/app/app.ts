import { Component, OnInit, signal } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { Api } from './services/api';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class App implements OnInit {
  protected readonly title = signal('frontend');

  constructor(private api: Api) {}

  ngOnInit() {
    this.api.getOne(4).subscribe(res => {
      console.log(res);
    });
  }

  load() {
    this.api.getOne(4).subscribe(res => {
      console.log(res);
    });
  }
}
