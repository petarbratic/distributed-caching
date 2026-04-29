import { Component, OnInit, signal } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { Api } from './services/api';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-root',
  imports: [FormsModule],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class App implements OnInit {
  protected readonly title = signal('frontend');

  id: number = 0;

  constructor(private api: Api) {}



  ngOnInit() {
    this.api.getOne(this.id).subscribe(res => {
      console.log(res);
    });
  }

  load() {
    this.api.getOne(this.id).subscribe(res => {
      console.log(res);
    });
  }
}
