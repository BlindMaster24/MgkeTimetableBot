import { describe, it, expect } from 'vitest';
import { ReadBuffer, WriteBuffer } from '../../src/utils/buffer';

describe('WriteBuffer + ReadBuffer', () => {
    it('roundtrips signed and unsigned ints in big-endian order', () => {
        const w = new WriteBuffer();
        w.writeInt8(-5);
        w.writeUInt8(250);
        w.writeInt16BE(-1000);
        w.writeUInt16BE(65000);
        w.writeInt32BE(-123456);
        w.writeUInt32BE(4_000_000_000);

        const r = new ReadBuffer(w.toBuffer());
        expect(r.readInt8()).toBe(-5);
        expect(r.readUInt8()).toBe(250);
        expect(r.readInt16BE()).toBe(-1000);
        expect(r.readUInt16BE()).toBe(65000);
        expect(r.readInt32BE()).toBe(-123456);
        expect(r.readUInt32BE()).toBe(4_000_000_000);
    });

    it('roundtrips little-endian variants and BigInt64', () => {
        const w = new WriteBuffer();
        w.writeUInt16LE(65000);
        w.writeInt32LE(-7);
        w.writeBigInt64LE(-10n);
        w.writeBigUInt64BE(1000n);

        const r = new ReadBuffer(w.toBuffer());
        expect(r.readUInt16LE()).toBe(65000);
        expect(r.readInt32LE()).toBe(-7);
        expect(r.readBigInt64LE()).toBe(-10n);
        expect(r.readBigUInt64BE()).toBe(1000n);
    });

    it('writeString/readString roundtrips UTF-8 strings', () => {
        const w = new WriteBuffer();
        const s = 'Привет, мир!';
        const bytes = Buffer.byteLength(s, 'utf8');
        w.writeString(s);

        const r = new ReadBuffer(w.toBuffer());
        expect(r.readString(bytes)).toBe(s);
    });

    it('writeZeroBytes produces the right length of zeros', () => {
        const w = new WriteBuffer();
        w.writeZeroBytes(5);
        const buf = w.toBuffer();

        expect(buf.length).toBe(5);
        expect(buf.equals(Buffer.alloc(5))).toBe(true);
    });

    it('ReadBuffer.skip advances the offset', () => {
        const buf = Buffer.from([1, 2, 3, 4, 5]);
        const r = new ReadBuffer(buf);

        r.skip(2);
        expect(r.readUInt8()).toBe(3);
    });

    it('readPadding returns all remaining bytes', () => {
        const buf = Buffer.from([1, 2, 3, 4, 5]);
        const r = new ReadBuffer(buf);
        r.skip(2);

        expect(r.readPadding().equals(Buffer.from([3, 4, 5]))).toBe(true);
    });

    it('explicit offset does not advance the read cursor', () => {
        const buf = Buffer.from([10, 20, 30]);
        const r = new ReadBuffer(buf);

        expect(r.readUInt8(2)).toBe(30);
        expect(r.readUInt8()).toBe(10);
    });
});
