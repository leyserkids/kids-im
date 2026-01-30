import { WSEvent } from '../types/entity';
import { CbEvents } from '../constant';
import { DataOfEvent } from '../types/eventData';
declare type Cbfn<E extends CbEvents> = (data: WSEvent<DataOfEvent<E>>) => void;
declare class Emitter {
    private events;
    constructor();
    emit<E extends CbEvents>(event: E, data: WSEvent<DataOfEvent<E>>): this;
    on<E extends CbEvents>(event: E, fn: Cbfn<E>): this;
    off<E extends CbEvents>(event: E, fn: Cbfn<E>): this | undefined;
}
export default Emitter;
