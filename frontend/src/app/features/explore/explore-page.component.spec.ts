import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { ExplorePageComponent } from './explore-page.component';
import { ExploreService, ExploreSection } from './explore.service';
import { PlayerService } from '../../core/services/player.service';
import { PlaylistService } from '../../core/services/playlist.service';
import { RadioService } from '../../core/services/radio.service';
import { StorageService } from '../../core/services/storage.service';
import { ToastService } from '../../core/services/toast.service';
import { LibraryService } from '../library/services/library.service';
import { PlayerStatus } from '../player/models/player.model';
import { Song } from '../../core/models/song.model';

describe('Explore playback feedback', () => {
  const song: Song = {id:'1',title:'Track',artist:'Artist',link:'https://example.com/1.mp3',image:''};
  const section: ExploreSection = {id:'test',title:'Test',subtitle:'',songs:[song]};
  const status = signal(PlayerStatus.Playing);
  const currentSong = signal<Song | null>(song);
  let component: ExplorePageComponent;
  let player: {status: typeof status; pause: jasmine.Spy; play: jasmine.Spy; playFromList: jasmine.Spy};
  beforeEach(() => {
    status.set(PlayerStatus.Playing); currentSong.set(song);
    player = {status, pause:jasmine.createSpy('pause'), play:jasmine.createSpy('play'), playFromList:jasmine.createSpy('playFromList')};
    TestBed.configureTestingModule({providers:[
      {provide:PlayerService,useValue:player}, {provide:PlaylistService,useValue:{currentSong}},
      {provide:ExploreService,useValue:{sections:signal([section])}}, {provide:RadioService,useValue:{}},
      {provide:StorageService,useValue:{}}, {provide:ToastService,useValue:{}}, {provide:LibraryService,useValue:{}}
    ]});
    component=TestBed.runInInjectionContext(()=>new ExplorePageComponent());
  });
  it('shows Pause and pauses the current song without resetting the queue',()=>{
    expect(component.actionLabel(song)).toBe('Pause Track by Artist');
    component.playFromShelf(section,song);
    expect(player.pause).toHaveBeenCalled();
    expect(player.playFromList).not.toHaveBeenCalled();
  });
  it('resumes paused tracks, ignores duplicate loading clicks and offers retry',()=>{
    status.set(PlayerStatus.Paused); component.playFromShelf(section,song);
    expect(player.play).toHaveBeenCalledTimes(1);
    status.set(PlayerStatus.Loading); component.playFromShelf(section,song);
    expect(player.play).toHaveBeenCalledTimes(1);
    expect(component.actionLabel(song)).toContain('Loading');
    status.set(PlayerStatus.Error); expect(component.actionLabel(song)).toContain('Retry');
    component.playFromShelf(section,song); expect(player.play).toHaveBeenCalledTimes(2);
  });
  it('starts another track with its shelf and keeps unrelated cards stopped',()=>{
    currentSong.set({...song,link:'https://example.com/other.mp3'});
    expect(component.statusFor(song)).toBe(PlayerStatus.Stopped);
    component.playFromShelf(section,song);
    expect(player.playFromList).toHaveBeenCalledWith(section.songs,song);
  });
  it('keeps the clicked quick pick visible when recommendation shelves change',()=>{
    component.playQuickPick(section,song);
    const explore=TestBed.inject(ExploreService);
    explore.sections.set([{...section,songs:[{...song,link:'https://example.com/new.mp3'}]}]);
    expect(component.featuredPicks()[0].song.link).toBe(song.link);
    expect(component.actionLabel(component.featuredPicks()[0].song)).toContain('Pause');
  });

});
