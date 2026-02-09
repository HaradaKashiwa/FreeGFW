import cn from 'classnames'
import { IoArrowForwardSharp } from 'react-icons/io5'
import { Button } from '@/components/ui/button'
import { useStartStore } from '../../store/useStartStore'
import { useNavigate } from 'react-router-dom'
import { PiSpinnerGap } from 'react-icons/pi'
import { Input } from '@/components/ui/input'
import { useLetsEncryptInit } from '../../apis/letsencrypt'
import { useState } from 'react'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
    DialogClose,
} from "@/components/ui/dialog"

export function LetsEncrypt() {
    const { setStep } = useStartStore()
    const [email, setEmail] = useState('')
    const [loading, setLoading] = useState(false)
    const { trigger: initLetsEncrypt, error: initError } = useLetsEncryptInit()

    return (
        <>
            <div className='md:flex justify-between items-center mt-8 @xs/modal:mt-0'>
                <div>
                    <div className='text-xl @xs/modal:hidden'>配置 SSL (Let's Encrypt)</div>
                    <div className='opacity-70 text-sm mt-2'>配置 SSL 证书以加密你的连接，如未配置您的连接信息可能会被窃取。</div>
                </div>
                <img src='/images/letsencrypt-logo-horizontal.svg' className='w-32 mt-3 md:mt-0' alt="Let's Encrypt" />
                {/* <div className="text-2xl font-bold opacity-50 @xs/modal:hidden">Let's Encrypt</div> */}
            </div>

            <div className='mt-8 space-y-4'>
                <div>
                    <label className='text-sm opacity-70 mb-1 block'>邮箱 (Email)</label>
                    <Input
                        type='email'
                        placeholder='your@email.com'
                        className='bg-white'
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                    />
                    <div className='text-xs mt-1 opacity-50'>
                        用于注册Let's Encrypt接收证书过期提醒。
                    </div>
                </div>
            </div>

            <div className='mt-8 flex justify-between'>
                <Dialog>
                    <DialogTrigger asChild>
                        <Button variant='destructive' className='cursor-pointer @xs/modal:hidden'>跳过加密</Button>
                    </DialogTrigger>
                    <DialogContent>
                        <DialogHeader>
                            <DialogTitle>确认跳过加密？</DialogTitle>
                            <DialogDescription>
                                跳过加密可能会收到政府或中间人攻击，导致信息泄漏。跳过加密将无法使用支持TLS的协议。
                            </DialogDescription>
                        </DialogHeader>
                        <DialogFooter>
                            <DialogClose asChild>
                                <Button variant="secondary">取消</Button>
                            </DialogClose>
                            <Button variant="destructive" onClick={() => setStep('select-user-type')}>确认跳过</Button>
                        </DialogFooter>
                    </DialogContent>
                </Dialog>
                <Button disabled={loading || !email} className='cursor-pointer' onClick={async () => {
                    try {
                        if (loading) return;
                        setLoading(true)
                        await initLetsEncrypt({ email })

                        // Wait for server restart and poll new domain
                        await new Promise(async resolve => {
                            while (true) {
                                try {
                                    // Try to fetch https version of the current host
                                    await fetch(`https://${window.location.hostname}:${window.location.port}`, { mode: 'no-cors' })
                                    // If no-cors fetch doesn't throw, it might mean connection established
                                    resolve(true)
                                    break;
                                } catch (e) {
                                    // ignore error and retry
                                }
                                await new Promise(resolve => setTimeout(resolve, 2000))
                            }
                        })
                        window.location.href = `https://${window.location.hostname}:${window.location.port}`
                    } finally {
                        setLoading(false)
                    }
                }}>加密我的连接 {loading ? <PiSpinnerGap className='animate-spin' /> : <IoArrowForwardSharp />}</Button>
            </div>
            {initError && <div className='bg-red-800 text-white p-4 mt-8 rounded-lg'>
                {initError?.message || '配置失败，请检查域名解析是否正确，或查看服务器日志。'}
            </div>}
        </>
    )
}

